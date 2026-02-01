package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type game struct {
	started atomic.Bool // locked once started

	turnCount    int64
	turnDuration time.Duration

	actionsLock sync.Mutex
	actionQueue chan *api.Action

	stateLock    sync.RWMutex
	stateHistory []*api.State
	currentState *api.State

	playersLock sync.RWMutex
	players     map[string]*player

	submittedActions map[string]map[string]*api.Action // map[playerID][cityName]action (last action wins)
	
	// Stalemate tracking
	lastOwnershipChange int64 // Turn number of last castle ownership change
}

func (s *game) run(ctx context.Context) {
	ticker := time.NewTicker(s.turnDuration)
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
			if !s.started.Load() {
				continue
			}

			// Timer expired, process the turn with whatever actions we have!
			actions := s.submittedActions
			s.submittedActions = make(map[string]map[string]*api.Action)

			s.stateLock.Lock()
			// Skip processing if game hasn't started or state is nil (e.g., after reset)
			if s.currentState == nil || !s.started.Load() {
				s.stateLock.Unlock()
				continue
			}

			if s.turnCount > 1_000_000 {
				s.stateLock.Unlock()
				return // you really suck at this game if it goes on for more than 1000 turns
			}
			s.turnCount++
			
			// Initialize lastOwnershipChange on first turn
			if s.turnCount == 1 {
				s.lastOwnershipChange = 1
			}
			
			// Save old state for comparison
			oldState := s.currentState
			
			nextState := processTurn(s.currentState, actions, s.turnCount)
			nextState.Turn = s.turnCount // Set turn on the new state before making it current
			s.stateHistory = append(s.stateHistory, s.currentState)
			s.currentState = nextState
			
			// Check for ownership changes by comparing old state to new state
			if s.turnCount > 1 {
				ownershipChanged := s.detectOwnershipChange(oldState, nextState)
				if ownershipChanged {
					log.Printf("Turn %d: Ownership changed, resetting stalemate counter", s.turnCount)
					s.lastOwnershipChange = s.turnCount
				}
			}
			
			victory := s.checkWinCondition()
			s.stateLock.Unlock()

			if victory {
				log.Println("Victory condition met, resetting game in 5 seconds...")
				time.Sleep(5 * time.Second)
			}
		}
	}
}

func (s *server) CreateGame(ctx context.Context, req *api.CreateGameRequest) (*api.CreateGameResponse, error) {
	newGameID := uuid.NewString()

	s.gamesLock.Lock()
	defer s.gamesLock.Unlock()

	s.games[newGameID] = create(s.turnDuration)

	return &api.CreateGameResponse{
		GameId: newGameID,
	}, nil
}

func (s *server) ListGames(ctx context.Context, req *api.ListGamesRequest) (*api.ListGamesResponse, error) {
	s.gamesLock.Lock()
	defer s.gamesLock.Unlock()

	return &api.ListGamesResponse{
		GameIds: slices.Collect(maps.Keys(s.games)),
	}, nil
}

func (s *game) start() error {
	s.playersLock.Lock()
	defer s.playersLock.Unlock()

	if len(s.players) < 2 {
		return status.Error(codes.FailedPrecondition, "not enough players to start the game")
	}

	if !s.started.CompareAndSwap(false, true) {
		return status.Error(codes.FailedPrecondition, "game already started")
	}

	s.initializeGameState()
	go s.run(context.Background())

	return nil
}

func create(turnDuration time.Duration) *game {
	return &game{
		turnDuration:        turnDuration,
		players:             make(map[string]*player),
		submittedActions:    make(map[string]map[string]*api.Action),
		actionQueue:         make(chan *api.Action, 100),
		lastOwnershipChange: 0,
	}
}

// initializeGameState creates the initial game state (separated for reuse)
func (s *game) initializeGameState() *game {
	// initialize the game state
	initialState := &api.State{
		Cities:      make(map[string]*api.City),
		Movements:   []*api.Movement{},
		Distances:   make(map[string]*api.Distance),
		PlayerNames: make(map[string]string),
	}

	for _, p := range s.players {
		// Store player name mapping
		initialState.PlayerNames[p.id] = p.name

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

	return s
}

// check victory condition: all cities owned by one player
func (s *game) checkWinCondition() bool {
	victor := ""
	for _, city := range s.currentState.Cities {
		if victor == "" {
			victor = city.Player
		} else if victor != city.Player {
			// Multiple players still in game, check for stalemate
			if s.turnCount-s.lastOwnershipChange >= 100 {
				// Stalemate: no ownership changes in 100 turns
				// Winner is player with most cities
				playerCityCounts := make(map[string]int)
				for _, c := range s.currentState.Cities {
					playerCityCounts[c.Player]++
				}
				
				maxCities := 0
				stalemateVictor := ""
				for playerID, count := range playerCityCounts {
					if count > maxCities {
						maxCities = count
						stalemateVictor = playerID
					}
				}
				
				fmt.Printf("Stalemate detected after %d turns without ownership change!\n", s.turnCount-s.lastOwnershipChange)
				fmt.Printf("Player %s wins by controlling %d cities (turn %d)!\n", stalemateVictor, maxCities, s.turnCount)
				
				// Write down history in json
				b, _ := json.Marshal(append(s.stateHistory, s.currentState))
				_ = os.WriteFile("gamehistory.json", b, 0o600)
				return true
			}
			return false
		}
	}
	
	// One player owns all cities
	fmt.Printf("Player %s has won the game in %d turns!\n", victor, s.turnCount)
	// write down history in json
	b, _ := json.Marshal(append(s.stateHistory, s.currentState))
	_ = os.WriteFile("gamehistory.json", b, 0o600)
	return true
}

// detectOwnershipChange compares two states and returns true if any city changed ownership
func (s *game) detectOwnershipChange(oldState, newState *api.State) bool {
	for cityName, newCity := range newState.Cities {
		oldCity, exists := oldState.Cities[cityName]
		if !exists || oldCity.Player != newCity.Player {
			return true
		}
	}
	return false
}

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

// BattleParticipant represents a player and their troops in a multi-player battle
type BattleParticipant struct {
	Player string
	Troops map[string]int64
}

// resolveMultiPlayerBattle resolves a battle with multiple players
// The first participant is assumed to be the city defender and gets a 2x power bonus
// Returns the winning player and their surviving troops
func resolveMultiPlayerBattle(participants []BattleParticipant) (winner string, survivingTroops map[string]int64) {
	const defenderBonus = 2.0 // City defender gets 2x power advantage
	// Filter out participants with no troops
	activePlayers := make([]BattleParticipant, 0, len(participants))
	for _, p := range participants {
		total := int64(0)
		for _, count := range p.Troops {
			total += count
		}
		if total > 0 {
			activePlayers = append(activePlayers, p)
		}
	}

	// Handle edge cases
	if len(activePlayers) == 0 {
		return "", make(map[string]int64)
	}
	if len(activePlayers) == 1 {
		return activePlayers[0].Player, activePlayers[0].Troops
	}

	// Calculate power for each player against all others combined
	powers := make([]float64, len(activePlayers))

	for i, player := range activePlayers {
		// Convert player's troops to array
		playerArmy := make([]float64, 3)
		playerTotal := 0.0
		for j, troopType := range troopOrder {
			playerArmy[j] = float64(player.Troops[troopType])
			playerTotal += playerArmy[j]
		}

		// Merge all other players' troops into one enemy army
		enemyArmy := make([]float64, 3)
		enemyTotal := 0.0
		for j, other := range activePlayers {
			if i == j {
				continue
			}
			for k, troopType := range troopOrder {
				troopCount := float64(other.Troops[troopType])
				enemyArmy[k] += troopCount
				enemyTotal += troopCount
			}
		}

		// Calculate effectiveness against merged enemy
		if playerTotal > 0 && enemyTotal > 0 {
			effectiveness := 0.0
			for pi := range 3 {
				for ei := range 3 {
					impact := troopImpact[troopOrder[pi]][troopOrder[ei]]
					effectiveness += playerArmy[pi] * impact * enemyArmy[ei]
				}
			}
			effectiveness /= (playerTotal * enemyTotal)
			powers[i] = effectiveness * playerTotal

			// Apply defender bonus to first participant (city owner)
			if i == 0 {
				powers[i] *= defenderBonus
			}
		} else {
			powers[i] = 0
		}
	}

	// Find winner (highest power)
	winnerIdx := 0
	maxPower := powers[0]
	for i := 1; i < len(powers); i++ {
		if powers[i] > maxPower {
			maxPower = powers[i]
			winnerIdx = i
		}
	}

	// Find second highest power for survivor calculation
	secondMaxPower := 0.0
	for i, power := range powers {
		if i != winnerIdx && power > secondMaxPower {
			secondMaxPower = power
		}
	}

	// Calculate surviving troops
	survivingTroops = make(map[string]int64)
	if maxPower > secondMaxPower {
		survivingRatio := (maxPower - secondMaxPower) / maxPower
		for _, troopType := range troopOrder {
			survivingTroops[troopType] = int64(float64(activePlayers[winnerIdx].Troops[troopType]) * survivingRatio)
		}
	} else {
		// Tie or very close - winner gets minimal survivors
		for _, troopType := range troopOrder {
			survivingTroops[troopType] = activePlayers[winnerIdx].Troops[troopType] / 10
		}
	}

	return activePlayers[winnerIdx].Player, survivingTroops
}
