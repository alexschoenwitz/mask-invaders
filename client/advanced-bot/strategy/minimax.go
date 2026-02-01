package strategy

import (
	"math"
	"time"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"google.golang.org/protobuf/proto"
)

// Minimax implements minimax search with alpha-beta pruning
type Minimax struct {
	playerID  string
	evaluator *Evaluator
	maxDepth  int
	timeLimit time.Duration
	startTime time.Time
}

// NewMinimax creates a new Minimax instance
func NewMinimax(playerID string, maxDepth int) *Minimax {
	return &Minimax{
		playerID:  playerID,
		evaluator: NewEvaluator(playerID),
		maxDepth:  maxDepth,
		timeLimit: 200 * time.Millisecond, // Leave buffer before 300ms deadline
	}
}

// SearchResult contains the best move and its score
type SearchResult struct {
	Actions []*api.Action
	Score   float64
	Depth   int
}

// Search performs minimax search to find the best move
func (m *Minimax) Search(state *api.State) SearchResult {
	m.startTime = time.Now()
	
	// Generate all possible moves for our cities
	moves := m.generateMoves(state, m.playerID)
	
	if len(moves) == 0 {
		return SearchResult{Actions: []*api.Action{}}
	}

	bestScore := math.Inf(-1)
	bestMoves := moves[0]

	alpha := math.Inf(-1)
	beta := math.Inf(1)

	for _, moveSet := range moves {
		// Apply moves to get next state
		nextState := m.applyMoves(state, moveSet, m.playerID)
		
		// Evaluate with minimax
		score := m.minimax(nextState, m.maxDepth-1, alpha, beta, false)
		
		if score > bestScore {
			bestScore = score
			bestMoves = moveSet
		}

		alpha = math.Max(alpha, score)
		
		// Check time limit
		if time.Since(m.startTime) > m.timeLimit {
			break
		}
	}

	return SearchResult{
		Actions: bestMoves,
		Score:   bestScore,
		Depth:   m.maxDepth,
	}
}

// minimax performs minimax search with alpha-beta pruning
func (m *Minimax) minimax(state *api.State, depth int, alpha, beta float64, maximizing bool) float64 {
	// Check time limit
	if time.Since(m.startTime) > m.timeLimit {
		return m.evaluator.Evaluate(state)
	}

	// Terminal conditions
	if depth == 0 {
		return m.evaluator.Evaluate(state)
	}

	// Check for game over
	gs := NewGameState(state, m.playerID)
	if len(gs.GetMyCities()) == 0 {
		return math.Inf(-1) // We lost
	}
	if len(gs.GetEnemyCities()) == 0 {
		return math.Inf(1) // We won
	}

	if maximizing {
		// Our turn
		maxEval := math.Inf(-1)
		moves := m.generateMoves(state, m.playerID)
		
		// If no moves, just evaluate current state
		if len(moves) == 0 {
			return m.evaluator.Evaluate(state)
		}

		for _, moveSet := range moves {
			nextState := m.applyMoves(state, moveSet, m.playerID)
			eval := m.minimax(nextState, depth-1, alpha, beta, false)
			maxEval = math.Max(maxEval, eval)
			alpha = math.Max(alpha, eval)
			if beta <= alpha {
				break // Beta cutoff
			}
			
			// Time check
			if time.Since(m.startTime) > m.timeLimit {
				break
			}
		}
		return maxEval
	} else {
		// Opponent's turn
		minEval := math.Inf(1)
		
		// Get all enemy players
		enemyPlayers := m.getEnemyPlayers(state)
		
		if len(enemyPlayers) == 0 {
			return m.evaluator.Evaluate(state)
		}

		// For simplicity, evaluate moves for first enemy
		// (full implementation would consider all enemies)
		enemyID := enemyPlayers[0]
		moves := m.generateMoves(state, enemyID)
		
		if len(moves) == 0 {
			return m.evaluator.Evaluate(state)
		}

		// Sample subset of moves to stay within time budget
		sampleSize := 5
		if len(moves) > sampleSize {
			moves = moves[:sampleSize]
		}

		for _, moveSet := range moves {
			nextState := m.applyMoves(state, moveSet, enemyID)
			eval := m.minimax(nextState, depth-1, alpha, beta, true)
			minEval = math.Min(minEval, eval)
			beta = math.Min(beta, eval)
			if beta <= alpha {
				break // Alpha cutoff
			}
			
			// Time check
			if time.Since(m.startTime) > m.timeLimit {
				break
			}
		}
		return minEval
	}
}

// generateMoves generates all possible moves for a player
func (m *Minimax) generateMoves(state *api.State, playerID string) [][]*api.Action {
	gs := NewGameState(state, playerID)
	cities := []*api.City{}
	
	for _, city := range state.Cities {
		if city.Player == playerID {
			cities = append(cities, city)
		}
	}

	if len(cities) == 0 {
		return [][]*api.Action{}
	}

	// Generate top moves for each city
	allMoves := [][]*api.Action{}
	
	// Strategy: Generate a few reasonable move combinations
	// 1. All cities build troops
	buildMoves := []*api.Action{}
	for _, city := range cities {
		buildMoves = append(buildMoves, &api.Action{
			Player: playerID,
			Action: &api.Action_CreateTroop{
				CreateTroop: &api.CreateTroop{
					In:   city.Id,
					Type: TroopA, // Simple choice
				},
			},
		})
	}
	allMoves = append(allMoves, buildMoves)

	// 2. Attack with strongest city
	if len(cities) > 0 {
		strongestCity := cities[0]
		maxTroops := GetTotalTroops(strongestCity.Troops)
		for _, city := range cities {
			troops := GetTotalTroops(city.Troops)
			if troops > maxTroops {
				maxTroops = troops
				strongestCity = city
			}
		}

		enemyCities := gs.GetEnemyCities()
		if len(enemyCities) > 0 && maxTroops > 5 {
			// Find closest enemy
			closestEnemy := enemyCities[0]
			minDist := gs.GetDistance(strongestCity.Id, closestEnemy.Id)
			
			for _, enemy := range enemyCities {
				dist := gs.GetDistance(strongestCity.Id, enemy.Id)
				if dist < minDist {
					minDist = dist
					closestEnemy = enemy
				}
			}

			attackMoves := []*api.Action{}
			// Attack with strongest city
			attackForce := make(map[string]int64)
			for troopType, count := range strongestCity.Troops {
				if count > 2 {
					attackForce[troopType] = count - 1
				}
			}

			if GetTotalTroops(attackForce) > 0 {
				attackMoves = append(attackMoves, &api.Action{
					Player: playerID,
					Action: &api.Action_Attack{
						Attack: &api.Attack{
							From: strongestCity.Id,
							To: &api.Attack_City{
								City: closestEnemy.Id,
							},
							Troops: attackForce,
						},
					},
				})

				// Others build
				for _, city := range cities {
					if city.Id != strongestCity.Id {
						attackMoves = append(attackMoves, &api.Action{
							Player: playerID,
							Action: &api.Action_CreateTroop{
								CreateTroop: &api.CreateTroop{
									In:   city.Id,
									Type: TroopB,
								},
							},
						})
					}
				}
				allMoves = append(allMoves, attackMoves)
			}
		}
	}

	// 3. Distributed attacks (if multiple cities)
	if len(cities) > 1 {
		enemyCities := gs.GetEnemyCities()
		if len(enemyCities) > 0 {
			distributedMoves := []*api.Action{}
			used := make(map[string]bool)
			
			for _, city := range cities {
				troops := GetTotalTroops(city.Troops)
				if troops > 5 {
					// Find an unused enemy target
					var target *api.City
					minDist := int64(999)
					
					for _, enemy := range enemyCities {
						if !used[enemy.Id] {
							dist := gs.GetDistance(city.Id, enemy.Id)
							if dist < minDist {
								minDist = dist
								target = enemy
							}
						}
					}

					if target != nil {
						used[target.Id] = true
						attackForce := make(map[string]int64)
						for troopType, count := range city.Troops {
							if count > 2 {
								attackForce[troopType] = count - 1
							}
						}

						if GetTotalTroops(attackForce) > 0 {
							distributedMoves = append(distributedMoves, &api.Action{
								Player: playerID,
								Action: &api.Action_Attack{
									Attack: &api.Attack{
										From: city.Id,
										To: &api.Attack_City{
											City: target.Id,
										},
										Troops: attackForce,
									},
								},
							})
						} else {
							// Not enough troops, build
							distributedMoves = append(distributedMoves, &api.Action{
								Player: playerID,
								Action: &api.Action_CreateTroop{
									CreateTroop: &api.CreateTroop{
										In:   city.Id,
										Type: TroopC,
									},
								},
							})
						}
					} else {
						// No targets left, build
						distributedMoves = append(distributedMoves, &api.Action{
							Player: playerID,
							Action: &api.Action_CreateTroop{
								CreateTroop: &api.CreateTroop{
									In:   city.Id,
									Type: TroopC,
								},
							},
						})
					}
				} else {
					// Too few troops, build
					distributedMoves = append(distributedMoves, &api.Action{
						Player: playerID,
						Action: &api.Action_CreateTroop{
							CreateTroop: &api.CreateTroop{
								In:   city.Id,
								Type: TroopA,
							},
						},
					})
				}
			}
			
			if len(distributedMoves) > 0 {
				allMoves = append(allMoves, distributedMoves)
			}
		}
	}

	return allMoves
}

// applyMoves applies a set of moves to a state and returns the new state
func (m *Minimax) applyMoves(state *api.State, moves []*api.Action, playerID string) *api.State {
	// Clone the state
	newState := proto.Clone(state).(*api.State)

	// Apply each action
	for _, action := range moves {
		switch a := action.Action.(type) {
		case *api.Action_CreateTroop:
			city := newState.Cities[a.CreateTroop.In]
			if city != nil && city.Player == playerID {
				if city.Troops == nil {
					city.Troops = make(map[string]int64)
				}
				city.Troops[a.CreateTroop.Type]++
			}

		case *api.Action_Attack:
			fromCity := newState.Cities[a.Attack.From]
			if fromCity != nil && fromCity.Player == playerID {
				// Remove troops from origin
				for troopType, count := range a.Attack.Troops {
					fromCity.Troops[troopType] -= count
					if fromCity.Troops[troopType] < 0 {
						fromCity.Troops[troopType] = 0
					}
				}
				
				// For simulation, immediately resolve the attack
				// (in reality it takes multiple turns)
				switch to := a.Attack.To.(type) {
				case *api.Attack_City:
					toCity := newState.Cities[to.City]
					if toCity != nil {
						result := SimulateBattle(a.Attack.Troops, toCity.Troops)
						if result.AttackerWins {
							toCity.Player = playerID
							toCity.Troops = result.SurvivingTroops
						} else {
							toCity.Troops = result.SurvivingTroops
						}
					}
				}
			}
		}
	}

	return newState
}

// getEnemyPlayers returns list of enemy player IDs
func (m *Minimax) getEnemyPlayers(state *api.State) []string {
	players := make(map[string]bool)
	for _, city := range state.Cities {
		if city.Player != m.playerID {
			players[city.Player] = true
		}
	}
	
	result := []string{}
	for playerID := range players {
		result = append(result, playerID)
	}
	return result
}
