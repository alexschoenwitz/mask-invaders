package main

import (
	"context"
	"fmt"
	"log"

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
				continue
			}
			if originCity.Player != playerID {
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
				// fmt.Printf("city %s no longer owned by %s, skipping action\n", createTroop.In, playerID)
				continue
			}

			// Add the troop
			city.Troops[createTroop.Type] += 1
		}
	}

	return state
}

func (s *server) StartGame(ctx context.Context, req *api.StartGameRequest) (*api.StartGameResponse, error) {
	g, ok := s.getGame(req.GameId)
	if !ok {
		return nil, status.Error(codes.NotFound, "game not found")
	}

	err := g.start()
	if err != nil {
		return nil, err
	}

	return &api.StartGameResponse{}, nil
}

// resetGameState resets the game state and clears players so they can register again
func (s *game) resetGameState() {
	s.stateLock.Lock()
	s.started.Store(false)
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

func (s *server) getGame(gameID string) (*game, bool) {
	s.gamesLock.RLock()
	defer s.gamesLock.RUnlock()

	g, ok := s.games[gameID]
	return g, ok
}

func (s *server) GetState(ctx context.Context, req *api.GetStateRequest) (*api.GetStateResponse, error) {
	g, ok := s.getGame(req.GameId)
	if !ok {
		return nil, status.Error(codes.NotFound, "game not found")
	}

	g.stateLock.RLock()
	defer g.stateLock.RUnlock()

	if g.currentState == nil {
		return &api.GetStateResponse{State: nil}, nil
	}

	// Must clone while holding the lock to prevent concurrent modification
	ret := proto.CloneOf(g.currentState)
	return &api.GetStateResponse{State: ret}, nil
}

func (s *server) GetStateHistory(ctx context.Context, req *api.GetStateHistoryRequest) (*api.GetStateHistoryResponse, error) {
	g, ok := s.getGame(req.GameId)
	if !ok {
		return nil, status.Error(codes.NotFound, "game not found")
	}

	g.stateLock.RLock()
	defer g.stateLock.RUnlock()

	ret := []*api.State{}
	for _, sh := range g.stateHistory {
		ret = append(ret, proto.CloneOf(sh))
	}

	return &api.GetStateHistoryResponse{States: ret}, nil
}

func (s *server) PostAction(ctx context.Context, req *api.PostActionRequest) (*api.PostActionResponse, error) {
	g, ok := s.getGame(req.GameId)
	if !ok {
		return nil, status.Error(codes.NotFound, "game not found")
	}

	token, ok := getPlayerToken(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "could not get player token from context")
	}

	g.playersLock.RLock()
	player := g.players[token]
	g.playersLock.RUnlock()

	if player.id != req.GetAction().GetPlayer() {
		return nil, status.Error(codes.PermissionDenied, "action player does not match token player")
	}

	g.stateLock.RLock()
	currentState := g.currentState
	g.stateLock.RUnlock()

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
		// State will be modified in processTurn, just validate here
	case *api.Action_None:
	default:
		return nil, status.Error(codes.InvalidArgument, "unknown action type")
	}

	// register the submitted action
	g.actionQueue <- req.GetAction()

	return &api.PostActionResponse{}, nil
}
