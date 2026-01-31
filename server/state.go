package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return
		case action := <-s.actionQueue:
			playerID := action.GetPlayer()

			// Get the city this action is for
			cityID := getCityFromAction(action)
			if cityID == "" {
				continue // Invalid action
			}

			// Initialize map if needed
			s.actionsLock.Lock()
			if s.submittedActions[playerID] == nil {
				s.submittedActions[playerID] = make(map[string]*api.Action)
			}

			// Store the action (last one wins - allows clients to update their guess)
			s.submittedActions[playerID][cityID] = action
			s.actionsLock.Unlock()

		case <-ticker.C:
			if !s.gameStarted.Load() {
				continue
			}

			// Timer expired, process the turn with whatever actions we have!
			s.actionsLock.Lock()
			actions := s.submittedActions
			s.submittedActions = make(map[string]map[string]*api.Action)
			s.actionsLock.Unlock()

			s.stateLock.Lock()
			// Skip processing if game hasn't started or state is nil (e.g., after reset)
			if s.currentState == nil || !s.gameStarted.Load() {
				s.stateLock.Unlock()
				continue
			}

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
				log.Println("Victory condition met, resetting game in 5 seconds...")
				time.Sleep(5 * time.Second)
				s.resetGameState()
				log.Println("Game state reset. Waiting for players to register again...")
			}
		}
	}
}

// getCityFromAction extracts the city name from an action
func getCityFromAction(action *api.Action) string {
	switch a := action.Action.(type) {
	case *api.Action_Attack:
		return a.Attack.From
	case *api.Action_CreateTroop:
		return a.CreateTroop.In
	case *api.Action_None:
		return "" // No specific city
	default:
		return ""
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

func processTurn(currentState *api.State, actions map[string]map[string]*api.Action, turnCount int64) *api.State {
	state := proto.CloneOf(currentState)

	// first process movements
	remainingMovements := make([]*api.Movement, 0, len(state.Movements))
	for _, movement := range state.Movements {
		if movement.ArrivingTurn != int64(turnCount) {
			remainingMovements = append(remainingMovements, movement)
			continue
		}

		// arrive at destination
		switch to := movement.To.(type) {
		case *api.Movement_City:
			city := state.Cities[to.City]
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
		case *api.Movement_Mine:
			// currently nothing happens when arriving at a mine
			mine, ok := state.Mines[to.Mine]
			if !ok {
				fmt.Printf("this should not happen, troops will just disappear\n")
				continue
			}
			// Claim the mine for the player if not already claimed
			if !mine.Claimed {
				mine.Claimed = true
			}
		default:
			fmt.Printf("unknown movement destination type, troops will just disappear\n")
		}
	}
	state.Movements = remainingMovements

	// then process player actions: ATTACKS FIRST
	for playerID, cityActions := range actions {
		for _, action := range cityActions {
			if action.GetAttack() == nil {
				continue
			}
			attack := action.GetAttack()

			// Re-validate ownership (city might have been captured this turn)
			originCity := state.Cities[attack.From]
			if originCity == nil {
				fmt.Printf("origin city %s does not exist, skipping action\n", attack.From)
				continue
			}
			if originCity.Player != playerID {
				fmt.Printf("city %s no longer owned by %s, skipping action\n", attack.From, playerID)
				continue
			}

			switch to := attack.GetTo().(type) {
			case *api.Attack_City:
				// create a new movement
				distance, ok := state.Distances[to.City+attack.From]
				if !ok {
					distance = state.Distances[attack.From+to.City]
				}
				if distance == nil {
					fmt.Printf("no distance found between %s and %s, skipping action\n", attack.From, to.City)
					continue
				}
				newMovement := &api.Movement{
					Player: playerID,
					From:   attack.From,
					To: &api.Movement_City{
						City: to.City,
					},
					Troops:       attack.Troops,
					ArrivingTurn: turnCount + distance.Distance,
				}
				state.Movements = append(state.Movements, newMovement)
			case *api.Attack_Mine:
				newMovement := &api.Movement{
					Player: playerID,
					From:   attack.From,
					To: &api.Movement_Mine{
						Mine: to.Mine,
					},
					Troops: attack.Troops,
					// distance to mine not implemented yet, assume 0 for now
				}
				state.Movements = append(state.Movements, newMovement)
			default:
				fmt.Printf("unknown attack destination type, skipping action\n")
				continue
			}

			// remove troops from the origin city
			for troopType, amount := range attack.Troops {
				newAmount := originCity.Troops[troopType] - amount
				originCity.Troops[troopType] = max(newAmount, 0)
			}
		}
	}

	// finally process CREATE TROOP actions
	for playerID, cityActions := range actions {
		for _, action := range cityActions {
			if action.GetCreateTroop() == nil {
				continue
			}
			createTroop := action.GetCreateTroop()

			// Re-validate ownership
			city := state.Cities[createTroop.In]
			if city == nil {
				fmt.Printf("city %s does not exist, skipping action\n", createTroop.In)
				continue
			}
			if city.Player != playerID {
				fmt.Printf("city %s no longer owned by %s, skipping action\n", createTroop.In, playerID)
				continue
			}

			// Add the troop
			city.Troops[createTroop.Type] += 1
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

	s.initializeGameState()

	return &api.StartGameResponse{}, nil
}

// initializeGameState creates the initial game state (separated for reuse)
func (s *server) initializeGameState() {
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
}

// resetGameState resets the game state and clears players so they can register again
// resetGameState resets the game state and clears players so they can register again
func (s *server) resetGameState() {
	s.stateLock.Lock()
	s.gameStarted.Store(false)
	s.currentState = nil
	s.stateHistory = []*api.State{}
	s.turnCount = 0
	s.stateLock.Unlock()

	s.actionsLock.Lock()
	s.submittedActions = make(map[string]map[string]*api.Action)
	s.actionsLock.Unlock()

	// Clear players so they can register again for the new game
	s.playersLock.Lock()
	s.players = make(map[string]*player)
	s.playersLock.Unlock()

	log.Println("All players cleared. Ready for new registrations.")
}

func (s *server) ResetGame(ctx context.Context, req *api.ResetGameRequest) (*api.ResetGameResponse, error) {
	s.playersLock.Lock()
	defer s.playersLock.Unlock()

	s.stateLock.Lock()
	defer s.stateLock.Unlock()

	s.gameStarted.Store(false)
	s.currentState = nil
	s.stateHistory = []*api.State{}
	s.turnCount = 0

	s.actionsLock.Lock()
	s.submittedActions = make(map[string]map[string]*api.Action)
	s.actionsLock.Unlock()

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
		switch to := attackAction.GetTo().(type) {
		case *api.Attack_City:
			toCity := currentState.Cities[to.City]
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
		case *api.Attack_Mine:
			// check mine existence and update claimed status
			mine, ok := currentState.Mines[to.Mine]
			if !ok {
				return nil, status.Error(codes.InvalidArgument, "invalid mine")
			}
			if mine.GetClaimed() {
				return nil, status.Error(codes.PermissionDenied, "mine already claimed")
			}
			mine.Claimed = true
			// add mine resources to player's city
			city, ok := currentState.Cities[attackAction.GetFrom()]
			if !ok || city.Player != player.id {
				return nil, status.Error(codes.PermissionDenied, "city not owned")
			}
			for resourceType, resource := range mine.GetResources() {
				cityResources, ok := city.Resources[resourceType]
				if !ok {
					cityResources = &api.Resource{Amount: 0}
					city.Resources[resourceType] = cityResources
				}
				cityResources.Amount += resource.GetAmount()
			}
		default:
			// valid
		}

	case *api.Action_CreateTroop:
		createTroopAction := req.GetAction().GetCreateTroop()
		if _, ok := validTroops[createTroopAction.GetType()]; !ok {
			return nil, status.Error(codes.InvalidArgument, "invalid troop type")
		}
		c, ok := currentState.Cities[createTroopAction.GetIn()]
		if !ok || c.Player != player.id {
			return nil, status.Error(codes.PermissionDenied, "city not owned")
		}
		// Don't modify state here anymore - will be done in processTurn
		c.Troops[createTroopAction.GetType()] += 1
	case *api.Action_None:
	default:
		return nil, status.Error(codes.InvalidArgument, "unknown action type")
	}

	// register the submitted action
	s.actionQueue <- req.GetAction()

	return &api.PostActionResponse{}, nil
}
