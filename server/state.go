package main

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

	// not used yet
	troopImpact = map[string]map[string]float64{
		troopA: {
			troopA: 1.0,
			troopB: 0.5,
			troopC: 2.0,
		},
		troopB: {
			troopA: 2.0,
			troopB: 1.0,
			troopC: 0.5,
		},
		troopC: {
			troopA: 0.5,
			troopB: 2.0,
			troopC: 1.0,
		},
	}
)

func (s *server) run(ctx context.Context) {
	ticker := time.NewTicker(200 * time.Millisecond)
	for {
		if s.turnCount > 1000 {
			return // you really suck at this game if it goes on for more than 1000 turns
		}

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
		s.turnCount++
		nextState := processTurn(s.currentState, actions, s.turnCount)
		s.stateHistory = append(s.stateHistory, s.currentState)
		s.currentState = nextState
		s.currentState.Turn = s.turnCount
		s.stateLock.Unlock()
	}
}

func processTurn(currentState *api.State, actions map[string]*api.Action, turnCount int64) *api.State {
	newState := &api.State{
		Cities:    make(map[string]*api.City),
		Movements: []*api.Movement{},
		Distances: currentState.Distances,
	}

	// first process movements
	for _, movement := range currentState.Movements {
		if movement.ArrivingTurn < int64(turnCount) {
			// still in transit
			newState.Movements = append(newState.Movements, movement)
			continue
		}

		// arrive at destination
		city := currentState.Cities[movement.To]
		if city == nil {
			fmt.Printf("this should not happen, troops will just disappear")
			continue
		}
		// TODO: calculate battle results and either conquer or not, but at the moment
		// we have 100% win rate for the attacker just because
		for troopType, ammount := range movement.Troops {
			city.Troops[troopType] = ammount
		}
		city.Player = movement.Player
		newState.Cities[movement.To] = city
	}

	// then process actions
	for playerID, action := range actions {
		if action.GetAttack() == nil {
			continue // we only support attack actions for now (none will skip anyway)
		}
		attack := action.GetAttack()
		// create a new movement
		distance, ok := currentState.Distances[attack.To+attack.From]
		if !ok {
			distance = currentState.Distances[attack.From+attack.To]
		}

		newMovement := &api.Movement{
			Player:       playerID,
			From:         attack.From,
			To:           attack.To,
			Troops:       attack.Troops,
			ArrivingTurn: turnCount + distance.Distance, // TODO: consider speed factors
		}
		newState.Movements = append(newState.Movements, newMovement)

		// remove troops from the origin city
		originCity := currentState.Cities[attack.From]
		if originCity == nil {
			fmt.Printf("this should not happen, troops will just disappear")
			continue
		}
		for troopType, ammount := range attack.Troops {
			originCity.Troops[troopType] -= ammount
		}
	}

	// finally, copy over unchanged cities
	for cityName, city := range currentState.Cities {
		if newState.Cities[cityName] == nil {
			newState.Cities[cityName] = city
		}
	}

	return newState
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
		// do a city per player with equal troops
		cityName := fmt.Sprintf("City-%s", p.id)
		initialState.Cities[cityName] = &api.City{
			Player: p.id,
			Troops: map[string]int64{
				troopA: 10,
				troopB: 10,
				troopC: 10,
			},
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
	s.stateLock.RLock()
	defer s.stateLock.RUnlock()

	token, ok := getPlayerToken(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "could not get player token from context")
	}
	if s.players[token].id != req.GetAction().GetPlayer() {
		return nil, status.Error(codes.PermissionDenied, "action player does not match token player")
	}

	// check if the action is valid
	switch req.GetAction().GetAction().(type) {
	case *api.Action_Attack:
		attackAction := req.GetAction().GetAttack()
		if attackAction.GetTroops() == nil {
			return nil, status.Error(codes.InvalidArgument, "troops must be defined")
		}
		if s.currentState.Cities[attackAction.GetFrom()] == nil || s.currentState.Cities[attackAction.GetTo()] == nil {
			return nil, status.Error(codes.InvalidArgument, "invalid from/to city")
		}
		for troopType, troopAmmount := range attackAction.GetTroops() {
			if _, ok := validTroops[troopType]; !ok {
				return nil, status.Errorf(codes.InvalidArgument, "invalid troop type: %s", troopType)
			}
			if troopAmmount <= 0 || s.currentState.Cities[attackAction.GetFrom()].GetTroops()[troopType] < troopAmmount {
				return nil, status.Errorf(codes.InvalidArgument, "invalid troop ammount for type %s: %d", troopType, troopAmmount)
			}
		}
	case *api.Action_None:
	default:
		return nil, status.Error(codes.InvalidArgument, "unknown action type")
	}

	// register the submitted action based on
	s.actionQueue <- req.GetAction()

	return &api.PostActionResponse{}, nil
}
