package strategy

import (
	"math"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
)

// Evaluator evaluates game states
type Evaluator struct {
	playerID string
}

// NewEvaluator creates a new Evaluator
func NewEvaluator(playerID string) *Evaluator {
	return &Evaluator{
		playerID: playerID,
	}
}

// EvaluationWeights defines scoring weights for evaluation
type EvaluationWeights struct {
	CityControl       float64
	MaterialAdvantage float64
	PositionalValue   float64
	ThreatPenalty     float64
	PowerAdvantage    float64
}

// DefaultWeights returns default evaluation weights
func DefaultWeights() EvaluationWeights {
	return EvaluationWeights{
		CityControl:       100.0,
		MaterialAdvantage: 10.0,
		PositionalValue:   5.0,
		ThreatPenalty:     20.0,
		PowerAdvantage:    15.0,
	}
}

// Evaluate evaluates a game state and returns a score (positive = good for player)
func (e *Evaluator) Evaluate(state *api.State) float64 {
	gs := NewGameState(state, e.playerID)
	weights := DefaultWeights()

	score := 0.0

	// 1. City Control (most important)
	myCities := gs.GetMyCities()
	enemyCities := gs.GetEnemyCities()
	cityDiff := len(myCities) - len(enemyCities)
	score += float64(cityDiff) * weights.CityControl

	// Check for win/loss
	if len(enemyCities) == 0 {
		return math.Inf(1) // We won!
	}
	if len(myCities) == 0 {
		return math.Inf(-1) // We lost!
	}

	// 2. Material Advantage (troop count)
	myTroops := gs.GetMyTotalTroops()
	enemyTroops := gs.GetEnemyTotalTroops()
	materialDiff := myTroops - enemyTroops
	score += float64(materialDiff) * weights.MaterialAdvantage

	// 3. Power Advantage (quality-adjusted troop strength)
	myPower := e.calculateAveragePower(gs, myCities, enemyCities)
	enemyPower := e.calculateAveragePower(gs, enemyCities, myCities)
	powerDiff := myPower - enemyPower
	score += powerDiff * weights.PowerAdvantage

	// 4. Positional Value (nearby enemies, strategic locations)
	positionalValue := e.calculatePositionalValue(gs, myCities, enemyCities)
	score += positionalValue * weights.PositionalValue

	// 5. Threat Assessment (incoming attacks)
	threatScore := e.calculateThreatScore(gs, myCities)
	score -= threatScore * weights.ThreatPenalty

	return score
}

// calculateAveragePower calculates average combat power of cities
func (e *Evaluator) calculateAveragePower(gs *GameState, myCities, enemyCities []*api.City) float64 {
	if len(myCities) == 0 || len(enemyCities) == 0 {
		return 0
	}

	totalPower := 0.0
	count := 0

	// Calculate average power against enemy cities
	for _, myCity := range myCities {
		for _, enemyCity := range enemyCities {
			power := CalculatePower(myCity.Troops, enemyCity.Troops)
			totalPower += power
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return totalPower / float64(count)
}

// calculatePositionalValue evaluates strategic positioning
func (e *Evaluator) calculatePositionalValue(gs *GameState, myCities, enemyCities []*api.City) float64 {
	value := 0.0

	for _, myCity := range myCities {
		// Reward cities that are close to enemies (offensive position)
		nearbyEnemies := 0
		totalDistance := int64(0)

		for _, enemyCity := range enemyCities {
			dist := gs.GetDistance(myCity.Id, enemyCity.Id)
			if dist < 10 {
				nearbyEnemies++
				totalDistance += dist
			}
		}

		if nearbyEnemies > 0 {
			avgDist := float64(totalDistance) / float64(nearbyEnemies)
			// Closer is better
			value += (10.0 - avgDist) * float64(nearbyEnemies)
		}
	}

	return value
}

// calculateThreatScore evaluates incoming threats
func (e *Evaluator) calculateThreatScore(gs *GameState, myCities []*api.City) float64 {
	threatScore := 0.0

	for _, city := range myCities {
		incoming := gs.GetIncomingMovements(city.Id)
		
		for _, mov := range incoming {
			if mov.Player != e.playerID {
				// Enemy incoming!
				myTroops := GetTotalTroops(city.Troops)
				
				// Simulate battle
				result := SimulateBattle(mov.Troops, city.Troops)
				
				if result.AttackerWins {
					// We'll lose this city!
					threatScore += 50.0
				} else {
					// We can defend but will lose troops
					casualties := myTroops - GetTotalTroops(result.SurvivingTroops)
					threatScore += float64(casualties) * 0.5
				}
				
				// Sooner threats are more urgent
				turnsAway := mov.ArrivingTurn - gs.State.Turn
				if turnsAway > 0 {
					urgency := 10.0 / float64(turnsAway)
					threatScore += urgency
				}
			}
		}
	}

	return threatScore
}

// CompareStates compares two states and returns score difference
func (e *Evaluator) CompareStates(state1, state2 *api.State) float64 {
	return e.Evaluate(state2) - e.Evaluate(state1)
}
