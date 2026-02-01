package strategy

import (
	"github.com/alexschoenwitz/mask-invaders/api/server/api"
)

// ThreatAnalysis contains information about threats to our cities
type ThreatAnalysis struct {
	City              *api.City
	IncomingAttacks   []*api.Movement
	TotalThreat       int64
	CanDefend         bool
	TurnsUntilImpact  int64
	RecommendedAction string
}

// AnalyzeThreats analyzes all threats to our cities
func AnalyzeThreats(gs *GameState) []*ThreatAnalysis {
	analyses := []*ThreatAnalysis{}

	for _, city := range gs.GetMyCities() {
		analysis := AnalyzeCityThreats(gs, city)
		if len(analysis.IncomingAttacks) > 0 {
			analyses = append(analyses, analysis)
		}
	}

	return analyses
}

// AnalyzeCityThreats analyzes threats to a specific city
func AnalyzeCityThreats(gs *GameState, city *api.City) *ThreatAnalysis {
	incoming := gs.GetIncomingMovements(city.Id)

	analysis := &ThreatAnalysis{
		City:             city,
		IncomingAttacks:  []*api.Movement{},
		TurnsUntilImpact: 999,
	}

	// Collect enemy attacks
	totalThreatTroops := make(map[string]int64)
	for _, mov := range incoming {
		if mov.Player != gs.PlayerID {
			analysis.IncomingAttacks = append(analysis.IncomingAttacks, mov)
			totalThreatTroops = AddTroops(totalThreatTroops, mov.Troops)

			turnsAway := mov.ArrivingTurn - gs.State.Turn
			if turnsAway < analysis.TurnsUntilImpact {
				analysis.TurnsUntilImpact = turnsAway
			}
		}
	}

	analysis.TotalThreat = GetTotalTroops(totalThreatTroops)

	if analysis.TotalThreat > 0 {
		// Simulate the battle
		result := SimulateBattle(totalThreatTroops, city.Troops)
		analysis.CanDefend = !result.AttackerWins

		if analysis.CanDefend {
			analysis.RecommendedAction = "FORTIFY"
		} else if analysis.TurnsUntilImpact > 3 {
			analysis.RecommendedAction = "COUNTER_ATTACK"
		} else {
			analysis.RecommendedAction = "EVACUATE"
		}
	}

	return analysis
}

// PrioritizeTargets prioritizes enemy cities for attack
func PrioritizeTargets(gs *GameState, fromCity *api.City) []*api.City {
	enemyCities := gs.GetEnemyCities()

	type targetScore struct {
		city  *api.City
		score float64
	}

	scores := []targetScore{}

	for _, enemy := range enemyCities {
		if !gs.CanReach(fromCity.Id, enemy.Id) {
			continue
		}

		score := scoreTarget(gs, fromCity, enemy)
		scores = append(scores, targetScore{city: enemy, score: score})
	}

	// Sort by score (descending)
	for i := 0; i < len(scores); i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].score > scores[i].score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	result := []*api.City{}
	for _, ts := range scores {
		result = append(result, ts.city)
	}

	return result
}

// scoreTarget scores an attack target
func scoreTarget(gs *GameState, fromCity, toCity *api.City) float64 {
	distance := gs.GetDistance(fromCity.Id, toCity.Id)
	myTroops := GetTotalTroops(fromCity.Troops)
	enemyTroops := GetTotalTroops(toCity.Troops)

	// CRITICAL: Check if others are already attacking this target
	incomingAttacks := gs.GetIncomingMovements(toCity.Id)
	otherPlayersTroops := make(map[string]int64)

	for _, mov := range incomingAttacks {
		if mov.Player != gs.PlayerID && mov.Player != toCity.Player {
			// Another player is attacking this target!
			// They will arrive first and fight, we'll fight the survivor
			otherPlayersTroops = AddTroops(otherPlayersTroops, mov.Troops)
		}
	}

	// If others are attacking, adjust our target's expected strength
	if GetTotalTroops(otherPlayersTroops) > 0 {
		// Simulate their battle
		otherResult := SimulateBattle(otherPlayersTroops, toCity.Troops)

		if otherResult.AttackerWins {
			// Other player will take the city, we'd be fighting them
			// This is now a bad target unless we can also beat them
			enemyTroops = GetTotalTroops(otherResult.SurvivingTroops)
		} else {
			// Defender survives but weakened
			enemyTroops = GetTotalTroops(otherResult.SurvivingTroops)
		}

		// Heavily penalize targets under contention
		// We're just feeding troops into a battle that might already be decided
		score := -500.0 // Strong penalty for contested targets
		return score
	}

	// Simulate battle with our troops
	result := SimulateBattle(fromCity.Troops, toCity.Troops)

	score := 0.0

	if result.AttackerWins {
		score += 100.0

		// Prefer closer targets
		score -= float64(distance) * 5.0

		// Prefer weaker targets (easier wins)
		troopDiff := myTroops - enemyTroops
		score += float64(troopDiff) * 2.0

		// Prefer high survival rate
		survivors := GetTotalTroops(result.SurvivingTroops)
		if myTroops > 0 {
			survivalRate := float64(survivors) / float64(myTroops)
			score += survivalRate * 50.0
		}

		// Bonus for targets under attack by others (finish them off)
		incoming := gs.GetIncomingMovements(toCity.Id)
		for _, mov := range incoming {
			if mov.Player != gs.PlayerID && mov.Player != toCity.Player {
				score += 25.0 // Coordinate with other attackers
			}
		}
	} else {
		// Can't win
		score = -1000.0
	}

	return score
}

// CoordinateAttacks finds opportunities for multi-city coordinated attacks
func CoordinateAttacks(gs *GameState, cities []*api.City) map[string][]*api.City {
	// Map of target -> attacking cities
	coordination := make(map[string][]*api.City)

	enemyCities := gs.GetEnemyCities()

	for _, enemy := range enemyCities {
		attackers := []*api.City{}
		combinedForce := make(map[string]int64)

		for _, myCity := range cities {
			if gs.CanReach(myCity.Id, enemy.Id) {
				troops := GetTotalTroops(myCity.Troops)
				if troops > 5 {
					attackers = append(attackers, myCity)
					combinedForce = AddTroops(combinedForce, myCity.Troops)
				}
			}
		}

		// If multiple cities can attack and combined force can win
		if len(attackers) >= 2 {
			result := SimulateBattle(combinedForce, enemy.Troops)
			if result.AttackerWins {
				coordination[enemy.Id] = attackers
			}
		}
	}

	return coordination
}

// ShouldExpand determines if we should focus on expansion
func ShouldExpand(gs *GameState) bool {
	myCities := len(gs.GetMyCities())
	enemyCities := len(gs.GetEnemyCities())

	// Expand if we have fewer cities
	if myCities < enemyCities {
		return true
	}

	// Early game: always expand
	if gs.State.Turn < 10 {
		return true
	}

	return false
}

// FindWeakestEnemy finds the weakest enemy city we can reach
func FindWeakestEnemy(gs *GameState, fromCity *api.City) *api.City {
	var weakest *api.City
	minTroops := int64(999999)

	for _, enemy := range gs.GetEnemyCities() {
		if gs.CanReach(fromCity.Id, enemy.Id) {
			troops := GetTotalTroops(enemy.Troops)
			if troops < minTroops {
				// Also check if we can win
				result := SimulateBattle(fromCity.Troops, enemy.Troops)
				if result.AttackerWins {
					minTroops = troops
					weakest = enemy
				}
			}
		}
	}

	return weakest
}
