package strategy

import (
	"log"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
)

// Strategy handles decision making for the bot
type Strategy struct {
	playerID      string
	evaluator     *Evaluator
	minimax       *Minimax
	useMinimaxAt  int64 // Turn number to start using minimax
	minimaxDepth  int
}

// NewStrategy creates a new Strategy instance
func NewStrategy(playerID string) *Strategy {
	return &Strategy{
		playerID:     playerID,
		evaluator:    NewEvaluator(playerID),
		useMinimaxAt: 5, // Use minimax after turn 5 (after initial expansion)
		minimaxDepth: 2, // Search 2 moves ahead
	}
}

// DecideActions decides what actions to take this turn
func (s *Strategy) DecideActions(state *api.State) []*api.Action {
	gs := NewGameState(state, s.playerID)
	
	actions := []*api.Action{}
	myCities := gs.GetMyCities()

	if len(myCities) == 0 {
		return actions // We lost
	}

	// Find the best target for ALL cities to attack (force concentration)
	bestTarget := s.findBestGlobalTarget(gs, myCities)

	// For each city, decide what to do
	for _, city := range myCities {
		action := s.decideCityAction(gs, city, bestTarget)
		if action != nil {
			actions = append(actions, action)
		}
	}

	return actions
}

// findBestGlobalTarget finds the best target for concentrated attack
func (s *Strategy) findBestGlobalTarget(gs *GameState, myCities []*api.City) *api.City {
	enemyCities := gs.GetEnemyCities()
	
	if len(enemyCities) == 0 {
		return nil
	}

	// Score each enemy for ALL our cities combined
	type targetScore struct {
		city  *api.City
		score float64
	}
	
	scores := []targetScore{}
	
	for _, enemy := range enemyCities {
		// CRITICAL: Check if this target is already under attack by others
		incomingAttacks := gs.GetIncomingMovements(enemy.Id)
		contested := false
		
		for _, mov := range incomingAttacks {
			if mov.Player != s.playerID && mov.Player != enemy.Player {
				// Another player is attacking - avoid this target!
				contested = true
				break
			}
		}
		
		if contested {
			// Skip heavily contested targets
			log.Printf("Skipping contested target %s", enemy.Id)
			continue
		}
		
		totalScore := 0.0
		reachable := 0
		
		for _, myCity := range myCities {
			if gs.CanReach(myCity.Id, enemy.Id) {
				reachable++
				// Closer cities contribute more to score
				dist := gs.GetDistance(myCity.Id, enemy.Id)
				totalScore += 100.0 / float64(dist+1)
			}
		}
		
		if reachable > 0 {
			// Prefer weaker enemies
			enemyTroops := GetTotalTroops(enemy.Troops)
			totalScore += 50.0 / float64(enemyTroops+1)
			
			// Bonus for multiple cities being able to attack
			totalScore += float64(reachable) * 20.0
			
			scores = append(scores, targetScore{city: enemy, score: totalScore})
		}
	}
	
	if len(scores) == 0 {
		// All targets contested, find the weakest uncontested enemy
		log.Printf("All targets contested, finding any uncontested target")
		for _, enemy := range enemyCities {
			incomingAttacks := gs.GetIncomingMovements(enemy.Id)
			contested := false
			
			for _, mov := range incomingAttacks {
				if mov.Player != s.playerID && mov.Player != enemy.Player {
					contested = true
					break
				}
			}
			
			if !contested {
				return enemy // Take first uncontested
			}
		}
		return nil // Everything is contested
	}
	
	// Find highest scoring target
	best := scores[0]
	for _, ts := range scores {
		if ts.score > best.score {
			best = ts
		}
	}
	
	log.Printf("Selected global target %s with score %.2f", best.city.Id, best.score)
	return best.city
}

// decideCityAction decides what action a single city should take
func (s *Strategy) decideCityAction(gs *GameState, city *api.City, globalTarget *api.City) *api.Action {
	cityTroops := GetTotalTroops(city.Troops)
	
	// Build threshold - need at least 8 troops before attacking
	buildThreshold := int64(8)
	
	// Analyze threats first
	threatAnalysis := AnalyzeCityThreats(gs, city)
	
	// If city is under serious threat, handle defense
	if len(threatAnalysis.IncomingAttacks) > 0 && !threatAnalysis.CanDefend {
		log.Printf("City %s under serious threat, defending", city.Id)
		return s.handleDefense(gs, city, threatAnalysis)
	}

	// If we have enough troops and a global target, attack it!
	if cityTroops >= buildThreshold && globalTarget != nil && gs.CanReach(city.Id, globalTarget.Id) {
		log.Printf("City %s attacking global target %s", city.Id, globalTarget.Id)
		return s.attackCityAggressive(gs, city, globalTarget)
	}

	// Otherwise, build troops
	return s.buildTroop(gs, city, globalTarget)
}

// handleDefense handles a city under threat
func (s *Strategy) handleDefense(gs *GameState, city *api.City, threatAnalysis *ThreatAnalysis) *api.Action {
	if threatAnalysis.CanDefend {
		// We can defend! Build counter troops
		totalThreat := make(map[string]int64)
		for _, attack := range threatAnalysis.IncomingAttacks {
			totalThreat = AddTroops(totalThreat, attack.Troops)
		}
		
		bestType := s.selectBestTroopType(gs, city, totalThreat)
		log.Printf("City %s defending with %s troops", city.Id, bestType)
		return &api.Action{
			Action: &api.Action_CreateTroop{
				CreateTroop: &api.CreateTroop{
					In:   city.Id,
					Type: bestType,
				},
			},
		}
	}

	// Can't defend!
	if threatAnalysis.TurnsUntilImpact > 3 {
		// Time to counter-attack
		log.Printf("City %s counter-attacking before impact", city.Id)
		targets := PrioritizeTargets(gs, city)
		if len(targets) > 0 {
			return s.attackCity(gs, city, targets[0])
		}
	}

	// Last resort: build defensive troops
	totalThreat := make(map[string]int64)
	for _, attack := range threatAnalysis.IncomingAttacks {
		totalThreat = AddTroops(totalThreat, attack.Troops)
	}
	bestType := s.selectBestTroopType(gs, city, totalThreat)
	log.Printf("City %s building emergency defense %s", city.Id, bestType)
	return &api.Action{
		Action: &api.Action_CreateTroop{
			CreateTroop: &api.CreateTroop{
				In:   city.Id,
				Type: bestType,
			},
		},
	}
}

// attackCity creates an attack action from one city to another
func (s *Strategy) attackCity(gs *GameState, fromCity, toCity *api.City) *api.Action {
	attackForce := s.calculateAttackForce(gs, fromCity, toCity)

	if GetTotalTroops(attackForce) > 0 {
		return &api.Action{
			Action: &api.Action_Attack{
				Attack: &api.Attack{
					From: fromCity.Id,
					To: &api.Attack_City{
						City: toCity.Id,
					},
					Troops: attackForce,
				},
			},
		}
	}

	// Can't attack, build instead
	return s.buildTroop(gs, fromCity)
}

// selectAttackTarget selects the best attack target for a city (legacy method)
func (s *Strategy) selectAttackTarget(gs *GameState, city *api.City) *api.Action {
	targets := PrioritizeTargets(gs, city)
	
	if len(targets) == 0 {
		return s.buildTroop(gs, city)
	}

	return s.attackCity(gs, city, targets[0])
}

// calculateAttackForce determines how many troops to send
func (s *Strategy) calculateAttackForce(gs *GameState, fromCity, toCity *api.City) map[string]int64 {
	myTroops := GetCityTroops(fromCity)
	enemyTroops := GetCityTroops(toCity)

	// Try different attack force sizes
	totalAvailable := GetTotalTroops(myTroops)
	
	// Keep some troops for defense (at least 3)
	reserve := int64(3)
	if totalAvailable <= reserve {
		return make(map[string]int64) // Not enough troops
	}

	attackSize := totalAvailable - reserve

	// Get optimal composition to counter enemy
	optimalComp := GetOptimalComposition(enemyTroops, attackSize)

	// Check if we have those troops available
	attackForce := make(map[string]int64)
	for troopType, needed := range optimalComp {
		available := myTroops[troopType]
		if available > 0 {
			// Use optimal composition but limited by what we have
			attackForce[troopType] = needed
		}
	}

	// If we don't have optimal composition, use what we have
	if GetTotalTroops(attackForce) == 0 {
		// Send mixed force, keeping reserve
		remaining := attackSize
		for _, troopType := range troopOrder {
			if remaining <= 0 {
				break
			}
			available := myTroops[troopType]
			if available > reserve/3 {
				send := available - reserve/3
				if send > remaining {
					send = remaining
				}
				attackForce[troopType] = send
				remaining -= send
			}
		}
	}

	// Verify we can win with this force
	result := SimulateBattle(attackForce, enemyTroops)
	if !result.AttackerWins {
		// Try with more troops
		attackForce = make(map[string]int64)
		for troopType, count := range myTroops {
			if count > 1 {
				attackForce[troopType] = count - 1
			}
		}
		
		// Check again
		result = SimulateBattle(attackForce, enemyTroops)
		if !result.AttackerWins {
			// Still can't win, don't attack
			return make(map[string]int64)
		}
	}

	return attackForce
}

// buildTroop decides which troop type to build
func (s *Strategy) buildTroop(gs *GameState, city *api.City, targetEnemy *api.City) *api.Action {
	var troopType string
	
	// If we have a target, build counters to their troops
	if targetEnemy != nil {
		optimal := GetOptimalComposition(targetEnemy.Troops, 1)
		for t := range optimal {
			troopType = t
			break
		}
	}
	
	// Fallback: build the type we have least of (balanced army)
	if troopType == "" {
		troopType = s.selectBestTroopType(gs, city, nil)
	}
	
	return &api.Action{
		Action: &api.Action_CreateTroop{
			CreateTroop: &api.CreateTroop{
				In:   city.Id,
				Type: troopType,
			},
		},
	}
}

// selectBestTroopType selects which troop type to build
func (s *Strategy) selectBestTroopType(gs *GameState, city *api.City, againstTroops map[string]int64) string {
	// If we're countering specific troops, choose the counter
	if againstTroops != nil && GetTotalTroops(againstTroops) > 0 {
		optimal := GetOptimalComposition(againstTroops, 1)
		for troopType := range optimal {
			return troopType
		}
	}

	// Build to balance our composition or counter average enemy
	myTroops := city.Troops
	minType := TroopA
	minCount := myTroops[TroopA]

	for _, troopType := range troopOrder {
		if myTroops[troopType] < minCount {
			minCount = myTroops[troopType]
			minType = troopType
		}
	}

	return minType
}
