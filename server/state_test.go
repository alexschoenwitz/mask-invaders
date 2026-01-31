package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Test Issue #3: Distance lookup missing validation - should handle gracefully
func TestProcessTurn_MissingDistance(t *testing.T) {
	currentState := &api.State{
		Cities: map[string]*api.City{
			"city1": {Player: "player1", Troops: map[string]int64{"A": 10}},
			"city2": {Player: "player2", Troops: map[string]int64{"A": 10}},
		},
		Distances: map[string]*api.Distance{}, // Empty - no distance defined
		Movements: []*api.Movement{},
	}

	actions := map[string]*api.Action{
		"player1": {
			Player: "player1",
			Action: &api.Action_Attack{
				Attack: &api.Attack{
					From:   "city1",
					To:     "city2",
					Troops: map[string]int64{"A": 5},
				},
			},
		},
	}

	// Should handle gracefully without panic
	nextState := processTurn(currentState, actions, 1)

	// No movement should be created
	if len(nextState.Movements) != 0 {
		t.Errorf("Expected no movements when distance is missing, got %d", len(nextState.Movements))
	}

	// Troops should remain unchanged
	if nextState.Cities["city1"].Troops["A"] != 10 {
		t.Errorf("Expected troops to remain at 10, got %d", nextState.Cities["city1"].Troops["A"])
	}
}

// Test Issue #4: Negative troop counts after multiple attacks
func TestProcessTurn_NegativeTroopCounts(t *testing.T) {
	currentState := &api.State{
		Cities: map[string]*api.City{
			"city1": {Player: "player1", Troops: map[string]int64{"A": 10}},
			"city2": {Player: "player2", Troops: map[string]int64{"A": 10}},
		},
		Distances: map[string]*api.Distance{
			"city1city2": {Distance: 1},
		},
		Movements: []*api.Movement{},
	}

	// Two actions trying to send more troops than available
	actions := map[string]*api.Action{
		"player1": {
			Player: "player1",
			Action: &api.Action_Attack{
				Attack: &api.Attack{
					From:   "city1",
					To:     "city2",
					Troops: map[string]int64{"A": 8},
				},
			},
		},
	}

	// First attack
	nextState := processTurn(currentState, actions, 1)

	// Try another attack with remaining troops
	actions2 := map[string]*api.Action{
		"player1": {
			Player: "player1",
			Action: &api.Action_Attack{
				Attack: &api.Attack{
					From:   "city1",
					To:     "city2",
					Troops: map[string]int64{"A": 8}, // Only 2 left, but asking for 8
				},
			},
		},
	}

	nextState2 := processTurn(nextState, actions2, 2)

	// Check if troops went negative
	troopCount := nextState2.Cities["city1"].Troops["A"]
	if troopCount < 0 {
		t.Errorf("Troop count went negative: %d", troopCount)
	}
}

// Test Issue #5: Battle tie handling (power1 == power2)
func TestCalculateBattle_Tie(t *testing.T) {
	// Equal forces should result in defender winning (as per current logic)
	attackingTroops := map[string]int64{"A": 10, "B": 10, "C": 10}
	defendingTroops := map[string]int64{"A": 10, "B": 10, "C": 10}

	attackerWins, _ := calculateBattle(attackingTroops, defendingTroops)

	if attackerWins {
		t.Error("Expected defender to win on tie, but attacker won")
	}
}

// Test Issue #7: Missing ownership validation
func TestPostAction_NoOwnershipCheck(t *testing.T) {
	s := &server{
		players: map[string]*player{
			"token1": {id: "player1", name: "Player 1"},
			"token2": {id: "player2", name: "Player 2"},
		},
		currentState: &api.State{
			Cities: map[string]*api.City{
				"city1": {Player: "player2", Troops: map[string]int64{"A": 10}}, // Owned by player2
				"city2": {Player: "player1", Troops: map[string]int64{"A": 10}},
			},
			Distances: map[string]*api.Distance{
				"city1city2": {Distance: 1},
			},
		},
		actionQueue: make(chan *api.Action, 10),
	}

	ctx := context.WithValue(context.Background(), playerTokenKey, "token1")

	// Player1 tries to attack FROM city1 (which they don't own)
	req := &api.PostActionRequest{
		Action: &api.Action{
			Player: "player1",
			Action: &api.Action_Attack{
				Attack: &api.Attack{
					From:   "city1", // player1 doesn't own this
					To:     "city2",
					Troops: map[string]int64{"A": 5},
				},
			},
		},
	}

	_, err := s.PostAction(ctx, req)

	// Currently this doesn't return an error - it should!
	if err != nil {
		// If we get an error, that's good - ownership is being checked
		if status.Code(err) != codes.InvalidArgument && status.Code(err) != codes.PermissionDenied {
			t.Errorf("Expected InvalidArgument or PermissionDenied error, got: %v", err)
		}
	} else {
		t.Error("Expected error when player attacks from city they don't own, but got none")
	}
}

// Test Issue #2: Race condition in ResetGame
func TestResetGame_RaceCondition(t *testing.T) {
	s := &server{
		players:          map[string]*player{"token1": {id: "player1"}},
		submittedActions: map[string]*api.Action{},
		actionQueue:      make(chan *api.Action, 10),
		currentState: &api.State{
			Cities:    map[string]*api.City{},
			Movements: []*api.Movement{},
			Distances: map[string]*api.Distance{},
		},
		stateHistory: []*api.State{},
	}
	s.gameStarted.Store(true)

	// Start the game loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.run(ctx)

	// Give run() time to start
	time.Sleep(50 * time.Millisecond)

	// Try to reset while run() is active
	_, err := s.ResetGame(context.Background(), &api.ResetGameRequest{})
	if err != nil {
		t.Fatalf("ResetGame failed: %v", err)
	}

	// Try to access submittedActions - if there's a race, this might panic
	time.Sleep(50 * time.Millisecond)

	// Send an action to trigger potential race
	action := &api.Action{Player: "player1"}
	select {
	case s.actionQueue <- action:
		// Action sent
	case <-time.After(100 * time.Millisecond):
		// Timeout is fine, we're just testing for races
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
}

// Test Issue #1: Race condition in run() - turnCount check outside lock
func TestRun_TurnCountRaceCondition(t *testing.T) {
	s := &server{
		players: map[string]*player{
			"token1": {id: "player1"},
			"token2": {id: "player2"},
		},
		submittedActions: map[string]*api.Action{},
		actionQueue:      make(chan *api.Action, 10),
		currentState: &api.State{
			Cities: map[string]*api.City{
				"city1": {Player: "player1", Troops: map[string]int64{"A": 10}},
				"city2": {Player: "player2", Troops: map[string]int64{"A": 10}},
			},
			Movements: []*api.Movement{},
			Distances: map[string]*api.Distance{},
		},
		stateHistory: []*api.State{},
		turnCount:    999, // Near the limit
	}
	s.gameStarted.Store(true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	// Start run() goroutine
	go func() {
		defer wg.Done()
		s.run(ctx)
	}()

	// Concurrently try to read turnCount
	go func() {
		defer wg.Done()
		for range 100 {
			s.stateLock.RLock()
			_ = s.turnCount
			s.stateLock.RUnlock()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Send actions to trigger turn processing
	for range 5 {
		s.actionQueue <- &api.Action{Player: "player1"}
		s.actionQueue <- &api.Action{Player: "player2"}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	wg.Wait()
}

// Test battle calculation with empty armies
func TestCalculateBattle_EmptyArmies(t *testing.T) {
	tests := []struct {
		name            string
		attackingTroops map[string]int64
		defendingTroops map[string]int64
		expectedWinner  bool // true = attacker, false = defender
	}{
		{
			name:            "both empty",
			attackingTroops: map[string]int64{},
			defendingTroops: map[string]int64{},
			expectedWinner:  true, // attacker wins by default
		},
		{
			name:            "attacker empty",
			attackingTroops: map[string]int64{},
			defendingTroops: map[string]int64{"A": 10},
			expectedWinner:  false,
		},
		{
			name:            "defender empty",
			attackingTroops: map[string]int64{"A": 10},
			defendingTroops: map[string]int64{},
			expectedWinner:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attackerWins, _ := calculateBattle(tt.attackingTroops, tt.defendingTroops)
			if attackerWins != tt.expectedWinner {
				t.Errorf("Expected attacker wins = %v, got %v", tt.expectedWinner, attackerWins)
			}
		})
	}
}

// Test concurrent access to PostAction
func TestPostAction_ConcurrentAccess(t *testing.T) {
	s := &server{
		players: map[string]*player{
			"token1": {id: "player1", name: "Player 1"},
			"token2": {id: "player2", name: "Player 2"},
		},
		currentState: &api.State{
			Cities: map[string]*api.City{
				"city1": {Player: "player1", Troops: map[string]int64{"A": 100}},
				"city2": {Player: "player2", Troops: map[string]int64{"A": 100}},
			},
			Distances: map[string]*api.Distance{
				"city1city2": {Distance: 1},
			},
		},
		actionQueue: make(chan *api.Action, 100),
	}

	var wg sync.WaitGroup
	errors := atomic.Int32{}

	// Send 50 concurrent requests
	for i := range 50 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			playerNum := (idx % 2) + 1
			token := "token" + string(rune('0'+playerNum))
			player := "player" + string(rune('0'+playerNum))
			from := "city" + string(rune('0'+playerNum))
			to := "city" + string(rune('0'+(3-playerNum)))

			ctx := context.WithValue(context.Background(), playerTokenKey, token)

			req := &api.PostActionRequest{
				Action: &api.Action{
					Player: player,
					Action: &api.Action_Attack{
						Attack: &api.Attack{
							From:   from,
							To:     to,
							Troops: map[string]int64{"A": 1},
						},
					},
				},
			}

			_, err := s.PostAction(ctx, req)
			if err != nil {
				errors.Add(1)
			}
		}(i)
	}

	wg.Wait()

	// Some errors are expected due to troop depletion, but shouldn't panic
	t.Logf("Errors encountered: %d/50", errors.Load())
}

// Test distance calculation in both directions
func TestProcessTurn_DistanceBothDirections(t *testing.T) {
	tests := []struct {
		name           string
		distances      map[string]*api.Distance
		from           string
		to             string
		expectMovement bool
		expectedTroops int64
	}{
		{
			name:           "forward direction exists",
			distances:      map[string]*api.Distance{"city1city2": {Distance: 5}},
			from:           "city1",
			to:             "city2",
			expectMovement: true,
			expectedTroops: 5,
		},
		{
			name:           "backward direction exists",
			distances:      map[string]*api.Distance{"city2city1": {Distance: 5}},
			from:           "city1",
			to:             "city2",
			expectMovement: true,
			expectedTroops: 5,
		},
		{
			name:           "no direction exists",
			distances:      map[string]*api.Distance{},
			from:           "city1",
			to:             "city2",
			expectMovement: false,
			expectedTroops: 10, // Troops should not be deducted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentState := &api.State{
				Cities: map[string]*api.City{
					"city1": {Player: "player1", Troops: map[string]int64{"A": 10}},
					"city2": {Player: "player2", Troops: map[string]int64{"A": 10}},
				},
				Distances: tt.distances,
				Movements: []*api.Movement{},
			}

			actions := map[string]*api.Action{
				"player1": {
					Player: "player1",
					Action: &api.Action_Attack{
						Attack: &api.Attack{
							From:   tt.from,
							To:     tt.to,
							Troops: map[string]int64{"A": 5},
						},
					},
				},
			}

			nextState := processTurn(currentState, actions, 1)

			if tt.expectMovement {
				if len(nextState.Movements) != 1 {
					t.Errorf("Expected 1 movement, got %d", len(nextState.Movements))
				}
				if nextState.Cities["city1"].Troops["A"] != 5 {
					t.Errorf("Expected 5 troops remaining, got %d", nextState.Cities["city1"].Troops["A"])
				}
			} else {
				if len(nextState.Movements) != 0 {
					t.Errorf("Expected no movements, got %d", len(nextState.Movements))
				}
				if nextState.Cities["city1"].Troops["A"] != tt.expectedTroops {
					t.Errorf("Expected %d troops, got %d", tt.expectedTroops, nextState.Cities["city1"].Troops["A"])
				}
			}
		})
	}
}
