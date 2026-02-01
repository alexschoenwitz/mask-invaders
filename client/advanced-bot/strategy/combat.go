package strategy

import (
	"github.com/alexschoenwitz/mask-invaders/api/server/api"
)

const (
	TroopA = "A"
	TroopB = "B"
	TroopC = "C"
)

var (
	troopOrder = []string{TroopA, TroopB, TroopC}

	// Combat effectiveness matrix
	troopImpact = map[string]map[string]float64{
		TroopA: {
			TroopA: 1.0,
			TroopB: 0.8,
			TroopC: 2.1,
		},
		TroopB: {
			TroopA: 1.3,
			TroopB: 1.0,
			TroopC: 0.9,
		},
		TroopC: {
			TroopA: 0.6,
			TroopB: 1.1,
			TroopC: 1.0,
		},
	}
)

// CombatResult represents the outcome of a battle
type CombatResult struct {
	AttackerWins    bool
	SurvivingTroops map[string]int64
	AttackerPower   float64
	DefenderPower   float64
}

// SimulateBattle simulates a battle between attacking and defending troops
// This matches the server's calculateBattle logic exactly
func SimulateBattle(attackingTroops, defendingTroops map[string]int64) CombatResult {
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
		return CombatResult{
			AttackerWins:    true,
			SurvivingTroops: make(map[string]int64),
		}
	}
	if total1 == 0 {
		return CombatResult{
			AttackerWins:    false,
			SurvivingTroops: copyTroops(defendingTroops),
		}
	}
	if total2 == 0 {
		return CombatResult{
			AttackerWins:    true,
			SurvivingTroops: copyTroops(attackingTroops),
		}
	}

	// 1. Calculate Combat Effectiveness (Quality)
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
	survivingTroops := make(map[string]int64)
	if power1 > power2 {
		// Attacker wins
		survivingRatio := (power1 - power2) / power1
		for i, troopType := range troopOrder {
			survivingTroops[troopType] = int64(army1[i] * survivingRatio)
		}
		return CombatResult{
			AttackerWins:    true,
			SurvivingTroops: survivingTroops,
			AttackerPower:   power1,
			DefenderPower:   power2,
		}
	} else {
		// Defender wins
		survivingRatio := (power2 - power1) / power2
		for i, troopType := range troopOrder {
			survivingTroops[troopType] = int64(army2[i] * survivingRatio)
		}
		return CombatResult{
			AttackerWins:    false,
			SurvivingTroops: survivingTroops,
			AttackerPower:   power1,
			DefenderPower:   power2,
		}
	}
}

// CalculatePower calculates the combat power of a troop composition against an opponent
func CalculatePower(myTroops, enemyTroops map[string]int64) float64 {
	myArmy := make([]float64, 3)
	enemyArmy := make([]float64, 3)
	
	for i, troopType := range troopOrder {
		myArmy[i] = float64(myTroops[troopType])
		enemyArmy[i] = float64(enemyTroops[troopType])
	}

	totalMy := 0.0
	totalEnemy := 0.0
	for i := range 3 {
		totalMy += myArmy[i]
		totalEnemy += enemyArmy[i]
	}

	if totalMy == 0 || totalEnemy == 0 {
		return totalMy
	}

	var eff float64
	for i := range 3 {
		for j := range 3 {
			impact := troopImpact[troopOrder[i]][troopOrder[j]]
			eff += myArmy[i] * impact * enemyArmy[j]
		}
	}
	eff /= (totalMy * totalEnemy)

	return eff * totalMy
}

// GetOptimalComposition finds the best troop composition to counter enemy troops
func GetOptimalComposition(enemyTroops map[string]int64, availableTroops int64) map[string]int64 {
	if availableTroops <= 0 {
		return make(map[string]int64)
	}

	// Count enemy troop types
	enemyA := enemyTroops[TroopA]
	enemyB := enemyTroops[TroopB]
	enemyC := enemyTroops[TroopC]

	// Strategy: Counter the dominant enemy type
	// A beats C, B beats A, C beats B
	result := make(map[string]int64)

	// Find dominant enemy type
	if enemyC >= enemyA && enemyC >= enemyB {
		// Enemy has mostly C, use A (A beats C with 2.1x)
		result[TroopA] = availableTroops
	} else if enemyA >= enemyB {
		// Enemy has mostly A, use B (B beats A with 1.3x)
		result[TroopB] = availableTroops
	} else {
		// Enemy has mostly B, use C (C beats B with 1.1x)
		result[TroopC] = availableTroops
	}

	return result
}

// GetTotalTroops returns the total number of troops
func GetTotalTroops(troops map[string]int64) int64 {
	total := int64(0)
	for _, count := range troops {
		total += count
	}
	return total
}

// SubtractTroops subtracts troops2 from troops1 (for sending troops)
func SubtractTroops(troops1, troops2 map[string]int64) map[string]int64 {
	result := copyTroops(troops1)
	for troopType, count := range troops2 {
		result[troopType] -= count
		if result[troopType] < 0 {
			result[troopType] = 0
		}
	}
	return result
}

// AddTroops adds two troop compositions together
func AddTroops(troops1, troops2 map[string]int64) map[string]int64 {
	result := copyTroops(troops1)
	for troopType, count := range troops2 {
		result[troopType] += count
	}
	return result
}

// copyTroops creates a deep copy of a troop map
func copyTroops(troops map[string]int64) map[string]int64 {
	result := make(map[string]int64)
	for k, v := range troops {
		result[k] = v
	}
	return result
}

// GetCityTroops safely gets troops from a city
func GetCityTroops(city *api.City) map[string]int64 {
	if city == nil || city.Troops == nil {
		return make(map[string]int64)
	}
	return copyTroops(city.Troops)
}
