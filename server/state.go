package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
)

const (
	troopA = "A"
	troopB = "B"
	troopC = "C"
)

var (
	validTroops = map[string]struct{}{
		troopA: {},
		troopB: {},
		troopC: {},
	}

	troopImpact = map[string]map[string]float64{
		troopA: {
			troopA: 1.0,
			troopB: 0.8,
			troopC: 2.1,
		},
		troopB: {
			troopA: 1.3,
			troopB: 1.0,
			troopC: 0.9,
		},
		troopC: {
			troopA: 0.6,
			troopB: 1.1,
			troopC: 1.0,
		},
	}

	troopOrder = []string{troopA, troopB, troopC}
)

func calculateBattle(attackingTroops, defendingTroops map[string]int64) (attackerWins bool, survivingTroops map[string]int64) {
	// Convert troops to arrays following troopOrder [A, B, C]
	army1 := make([]float64, 3)
	army2 := make([]float64, 3)
	for i, troopType := range troopOrder {
		army1[i] = float64(attackingTroops[troopType])
		army2[i] = float64(defendingTroops[troopType])
	}

	// Calculate total troop counts
	total1 := 0.0
	total2 := 0.0
	for i := range 3 {
		total1 += army1[i]
		total2 += army2[i]
	}

	// Handle edge cases
	if total1 == 0 && total2 == 0 {
		return true, make(map[string]int64)
	}
	if total1 == 0 {
		return false, defendingTroops
	}
	if total2 == 0 {
		return true, attackingTroops
	}

	// 1. Calculate Combat Effectiveness (Quality)
	// eff_1 = (army1 @ D @ army2) / (total1 * total2)
	var eff1, eff2 float64
	for i := range 3 {
		for j := range 3 {
			impact := troopImpact[troopOrder[i]][troopOrder[j]]
			eff1 += army1[i] * impact * army2[j]
			eff2 += army2[i] * impact * army1[j]
		}
	}
	eff1 /= (total1 * total2)
	eff2 /= (total1 * total2)

	// 2. Linear Law: Power = Quality * Quantity
	power1 := eff1 * total1
	power2 := eff2 * total2

	// 3. Determine winner and calculate survivors
	survivingTroops = make(map[string]int64)
	if power1 > power2 {
		// Attacker wins
		survivingRatio := (power1 - power2) / power1
		for i, troopType := range troopOrder {
			survivingTroops[troopType] = int64(army1[i] * survivingRatio)
		}
		return true, survivingTroops
	} else {
		// Defender wins
		survivingRatio := (power2 - power1) / power2
		for i, troopType := range troopOrder {
			survivingTroops[troopType] = int64(army2[i] * survivingRatio)
		}
		return false, survivingTroops
	}
}

func (s *server) run(ctx context.Context) {
	ticker := time.NewTicker(200 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return
		case action := <-s.actionQueue:
			s.submittedActions[action.GetPlayer()] = action
			if len(s.submittedActions) != len(s.players) {
				continue
			}
		case <-ticker.C:
			if !s.gameStarted.Load() {
				continue
			}
		}
		// every player submitted an action, or the timer expired, process the turn!
		s.actionsLock.Lock()
		actions := s.submittedActions
		s.submittedActions = make(map[string]*api.Action)
		s.actionsLock.Unlock()

		s.stateLock.Lock()
		if s.turnCount > 1000 {
			s.stateLock.Unlock()
			return // you really suck at this game if it goes on for more than 1000 turns
		}
		s.turnCount++
		nextState := processTurn(s.currentState, actions, s.turnCount)
		s.stateHistory = append(s.stateHistory, s.currentState)
		s.currentState = nextState
		s.currentState.Turn = s.turnCount
		victory := s.checkWinCondition()
		s.stateLock.Unlock()

		if victory {
			time.Sleep(2 * time.Second) // be nice enough for players to be able to read the victory state
			s.shutdown()
		}
	}
}

// check victory condition: all cities owned by one player
func (s *server) checkWinCondition() bool {
	victor := ""
	for _, city := range s.currentState.Cities {
		if victor == "" {
			victor = city.Player
		} else if victor != city.Player {
			return false
		}
	}
	fmt.Printf("Player %s has won the game in %d turns!\n", victor, s.turnCount)
	// write down history in json
	b, _ := json.Marshal(append(s.stateHistory, s.currentState))
	_ = os.WriteFile("gamehistory.json", b, 0o600)
	return true
}

func processTurn(currentState *api.State, actions map[string]*api.Action, turnCount int64) *api.State {
	state := proto.CloneOf(currentState)

	// first process movements
	remainingMovements := make([]*api.Movement, 0, len(state.Movements))
	for _, movement := range state.Movements {
		if movement.ArrivingTurn != int64(turnCount) {
			remainingMovements = append(remainingMovements, movement)
			continue
		}

		// arrive at destination
		city := state.Cities[movement.To]
		if city == nil {
			fmt.Printf("this should not happen, troops will just disappear")
			continue
		}

		// Battle!
		attackerWins, survivingTroops := calculateBattle(movement.Troops, city.Troops)
		if attackerWins {
			city.Player = movement.Player
			city.Troops = survivingTroops
		} else {
			city.Troops = survivingTroops
		}
	}
	state.Movements = remainingMovements

	// then process actions
	for playerID, action := range actions {
		if action.GetAttack() == nil {
			continue // we only support attack actions for now (none will skip anyway)
		}
		attack := action.GetAttack()
		// create a new movement
		distance, ok := state.Distances[attack.To+attack.From]
		if !ok {
			distance = state.Distances[attack.From+attack.To]
		}
		if distance == nil {
			fmt.Printf("no distance found between %s and %s, skipping action\n", attack.From, attack.To)
			continue
		}

		newMovement := &api.Movement{
			Player:       playerID,
			From:         attack.From,
			To:           attack.To,
			Troops:       attack.Troops,
			ArrivingTurn: turnCount + distance.Distance, // TODO: consider speed factors
		}
		state.Movements = append(state.Movements, newMovement)

		// remove troops from the origin city
		originCity := state.Cities[attack.From]
		if originCity == nil {
			fmt.Printf("this should not happen, troops will just disappear")
			continue
		}
		for troopType, amount := range attack.Troops {
			newAmount := originCity.Troops[troopType] - amount
			originCity.Troops[troopType] = max(newAmount, 0)
		}
	}

	return state
}

func (s *server) StartGame(ctx context.Context, req *api.StartGameRequest) (*api.StartGameResponse, error) {
	s.playersLock.Lock()
	defer s.playersLock.Unlock()

	if len(s.players) < 2 {
		return nil, status.Error(codes.FailedPrecondition, "not enough players to start the game")
	}

	if !s.gameStarted.CompareAndSwap(false, true) {
		return nil, status.Error(codes.FailedPrecondition, "game already started")
	}

	// initialize the game state
	initialState := &api.State{
		Cities:    make(map[string]*api.City),
		Movements: []*api.Movement{},
		Distances: make(map[string]*api.Distance),
	}

	for _, p := range s.players {
		// do a 3 cities per player with equal troops
		for i := range 3 {
			cityName := fmt.Sprintf("City-%s-%d", p.id, i)
			initialState.Cities[cityName] = &api.City{
				Player: p.id,
				Troops: map[string]int64{
					troopA: 10,
					troopB: 10,
					troopC: 10,
				},
			}
		}
	}

	// define some distances between cities with randomness for variety between 4 and 8 turns
	cityNames := []string{}
	for cityName := range initialState.Cities {
		cityNames = append(cityNames, cityName)
	}
	for i := 0; i < len(cityNames); i++ {
		for j := i + 1; j < len(cityNames); j++ {
			distance := int64(4 + (i+j)%5) // pseudo-random for now
			initialState.Distances[cityNames[i]+cityNames[j]] = &api.Distance{
				Edge:     cityNames[i] + cityNames[j],
				Distance: distance,
			}
		}
	}

	s.currentState = initialState
	s.stateHistory = []*api.State{}
	s.turnCount = 0

	return &api.StartGameResponse{}, nil
}

func (s *server) ResetGame(ctx context.Context, req *api.ResetGameRequest) (*api.ResetGameResponse, error) {
	s.playersLock.Lock()
	defer s.playersLock.Unlock()

	s.gameStarted.Store(false)
	s.currentState = nil
	s.stateHistory = []*api.State{}
	s.turnCount = 0
	s.submittedActions = make(map[string]*api.Action)
	s.players = make(map[string]*player)

	return &api.ResetGameResponse{}, nil
}

func (s *server) GetState(ctx context.Context, req *api.GetStateRequest) (*api.GetStateResponse, error) {
	s.stateLock.RLock()
	defer s.stateLock.RUnlock()

	return &api.GetStateResponse{State: s.currentState}, nil
}

func (s *server) GetStateHistory(ctx context.Context, req *api.GetStateHistoryRequest) (*api.GetStateHistoryResponse, error) {
	s.stateLock.RLock()
	defer s.stateLock.RUnlock()

	return &api.GetStateHistoryResponse{States: s.stateHistory}, nil
}

func (s *server) PostAction(ctx context.Context, req *api.PostActionRequest) (*api.PostActionResponse, error) {
	token, ok := getPlayerToken(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "could not get player token from context")
	}

	s.playersLock.RLock()
	player := s.players[token]
	s.playersLock.RUnlock()

	if player.id != req.GetAction().GetPlayer() {
		return nil, status.Error(codes.PermissionDenied, "action player does not match token player")
	}

	s.stateLock.RLock()
	currentState := s.currentState
	s.stateLock.RUnlock()

	// check if the action is valid
	switch req.GetAction().GetAction().(type) {
	case *api.Action_Attack:
		attackAction := req.GetAction().GetAttack()
		if attackAction.GetTroops() == nil {
			return nil, status.Error(codes.InvalidArgument, "troops must be defined")
		}
		fromCity := currentState.Cities[attackAction.GetFrom()]
		toCity := currentState.Cities[attackAction.GetTo()]
		if fromCity == nil || toCity == nil {
			return nil, status.Error(codes.InvalidArgument, "invalid from/to city")
		}
		// Check ownership of the from city
		if fromCity.GetPlayer() != player.id {
			return nil, status.Error(codes.PermissionDenied, "cannot attack from city you don't own")
		}
		for troopType, troopAmount := range attackAction.GetTroops() {
			if _, ok := validTroops[troopType]; !ok {
				return nil, status.Errorf(codes.InvalidArgument, "invalid troop type: %s", troopType)
			}
			if troopAmount <= 0 || fromCity.GetTroops()[troopType] < troopAmount {
				return nil, status.Errorf(codes.InvalidArgument, "invalid troop amount for type %s: %d", troopType, troopAmount)
			}
		}
	case *api.Action_None:
	default:
		return nil, status.Error(codes.InvalidArgument, "unknown action type")
	}

	// register the submitted action
	s.actionQueue <- req.GetAction()

	return &api.PostActionResponse{}, nil
}
