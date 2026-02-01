package strategy

import (
	"github.com/alexschoenwitz/mask-invaders/api/server/api"
)

// GameState wraps the API state with helper methods
type GameState struct {
	State    *api.State
	PlayerID string
}

// NewGameState creates a new GameState wrapper
func NewGameState(state *api.State, playerID string) *GameState {
	return &GameState{
		State:    state,
		PlayerID: playerID,
	}
}

// GetMyCities returns all cities owned by the player
func (gs *GameState) GetMyCities() []*api.City {
	cities := []*api.City{}
	for _, city := range gs.State.Cities {
		if city.Player == gs.PlayerID {
			cities = append(cities, city)
		}
	}
	return cities
}

// GetEnemyCities returns all cities not owned by the player
func (gs *GameState) GetEnemyCities() []*api.City {
	cities := []*api.City{}
	for _, city := range gs.State.Cities {
		if city.Player != gs.PlayerID {
			cities = append(cities, city)
		}
	}
	return cities
}

// GetCity returns a city by ID
func (gs *GameState) GetCity(cityID string) *api.City {
	return gs.State.Cities[cityID]
}

// GetDistance returns the distance between two cities
func (gs *GameState) GetDistance(city1, city2 string) int64 {
	dist := gs.State.Distances[city1+city2]
	if dist == nil {
		dist = gs.State.Distances[city2+city1]
	}
	if dist == nil {
		return 999 // No connection
	}
	return dist.Distance
}

// GetIncomingMovements returns all movements arriving at a city
func (gs *GameState) GetIncomingMovements(cityID string) []*api.Movement {
	movements := []*api.Movement{}
	for _, mov := range gs.State.Movements {
		if movCity, ok := mov.To.(*api.Movement_City); ok {
			if movCity.City == cityID {
				movements = append(movements, mov)
			}
		}
	}
	return movements
}

// GetMyMovements returns all movements from this player
func (gs *GameState) GetMyMovements() []*api.Movement {
	movements := []*api.Movement{}
	for _, mov := range gs.State.Movements {
		if mov.Player == gs.PlayerID {
			movements = append(movements, mov)
		}
	}
	return movements
}

// GetEnemyMovements returns all movements from other players
func (gs *GameState) GetEnemyMovements() []*api.Movement {
	movements := []*api.Movement{}
	for _, mov := range gs.State.Movements {
		if mov.Player != gs.PlayerID {
			movements = append(movements, mov)
		}
	}
	return movements
}

// IsUnderThreat checks if a city is under immediate threat (incoming enemy troops)
func (gs *GameState) IsUnderThreat(cityID string) bool {
	city := gs.GetCity(cityID)
	if city == nil || city.Player != gs.PlayerID {
		return false
	}

	incoming := gs.GetIncomingMovements(cityID)
	for _, mov := range incoming {
		if mov.Player != gs.PlayerID {
			return true
		}
	}
	return false
}

// GetTotalTroopsInCity returns total troops in a city including type
func (gs *GameState) GetTotalTroopsInCity(cityID string) int64 {
	city := gs.GetCity(cityID)
	if city == nil {
		return 0
	}
	return GetTotalTroops(city.Troops)
}

// CanReach checks if city1 can reach city2 (there's a distance defined)
func (gs *GameState) CanReach(city1, city2 string) bool {
	return gs.GetDistance(city1, city2) < 999
}

// GetAllCityIDs returns all city IDs
func (gs *GameState) GetAllCityIDs() []string {
	ids := []string{}
	for id := range gs.State.Cities {
		ids = append(ids, id)
	}
	return ids
}

// GetMyTotalTroops returns the total number of troops across all player cities
func (gs *GameState) GetMyTotalTroops() int64 {
	total := int64(0)
	for _, city := range gs.GetMyCities() {
		total += GetTotalTroops(city.Troops)
	}
	return total
}

// GetEnemyTotalTroops returns the total number of troops across all enemy cities
func (gs *GameState) GetEnemyTotalTroops() int64 {
	total := int64(0)
	for _, city := range gs.GetEnemyCities() {
		total += GetTotalTroops(city.Troops)
	}
	return total
}

// GetNearbyCities returns cities within a certain distance
func (gs *GameState) GetNearbyCities(fromCity string, maxDistance int64) []string {
	nearby := []string{}
	for _, cityID := range gs.GetAllCityIDs() {
		if cityID != fromCity {
			dist := gs.GetDistance(fromCity, cityID)
			if dist <= maxDistance {
				nearby = append(nearby, cityID)
			}
		}
	}
	return nearby
}
