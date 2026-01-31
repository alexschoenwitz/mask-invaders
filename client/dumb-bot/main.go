package main

import (
	"log"
	"math"
	"time"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"github.com/alexschoenwitz/mask-invaders/client"
)

const (
	serverURL = "http://localhost:8080"
	botName   = "DumbBot"
)

func main() {
	// 1. Register the bot
	c := client.NewClient(serverURL)
	playerID, err := c.Register(botName)
	if err != nil {
		log.Fatalf("Failed to register: %v", err)
	}
	log.Printf("Registered with playerID: %s", playerID)

	// 2. Try to start the game for 2 minutes
	gameStarted := false
	startTime := time.Now()

	for time.Since(startTime) < 2*time.Minute {
		err := c.StartGame()
		if err == nil {
			log.Println("Game started successfully!")
			gameStarted = true
			break
		}
		log.Printf("Failed to start game: %v. Retrying in 1 seconds...", err)
		time.Sleep(1 * time.Second)
	}

	if !gameStarted {
		log.Println("Failed to start game within 2 minutes. Giving up.")
		return
	}

	// 3. Play the game with the dumb strategy
	playGame(c, playerID)
}

func playGame(client *client.Client, playerID string) {
	log.Println("Starting game loop...")

	for {
		time.Sleep(1 * time.Second) // Give some time between turns

		state, err := client.GetState()
		if err != nil {
			log.Printf("Failed to get game state: %v", err)
			continue
		}

		// Find my cities and enemy cities
		myCities := []string{}
		enemyCities := []string{}

		for cityName, city := range state.Cities {
			if city.Player == playerID {
				myCities = append(myCities, cityName)
			} else {
				enemyCities = append(enemyCities, cityName)
			}
		}

		if len(myCities) == 0 {
			log.Println("No cities left, game over!")
			break
		}

		if len(enemyCities) == 0 {
			log.Println("All cities conquered, victory!")
			break
		}

		// NEW: Take action for each city
		for _, cityName := range myCities {
			city := state.Cities[cityName]

			if !hasTroops(city) {
				log.Printf("City %s has no troops, skipping", cityName)
				continue
			}

			// Find the nearest enemy city to attack from this city
			attackTo := findNearestEnemyFrom(cityName, enemyCities, state)
			if attackTo == "" {
				log.Printf("City %s: No valid attack target found", cityName)
				continue
			}

			// Attack with all troops from this city
			action := &api.PostActionRequest{
				Action: &api.Action{
					Player: playerID,
					Action: &api.Action_Attack{
						Attack: &api.Attack{
							From:   cityName,
							To:     &api.Attack_City{City: attackTo},
							Troops: city.Troops,
						},
					},
				},
			}

			err = client.PostAction(action)
			if err != nil {
				log.Printf("City %s: Failed to attack %s: %v", cityName, attackTo, err)
			} else {
				log.Printf("City %s: Attacking %s", cityName, attackTo)
			}
		}
	}
}

func hasTroops(c *api.City) bool {
	for _, ammount := range c.Troops {
		if ammount > 0 {
			return true
		}
	}
	return false
}

func findNearestEnemyFrom(myCity string, enemyCities []string, state *api.State) string {
	minDistance := int64(math.MaxInt64)
	var attackTo string

	for _, enemyCity := range enemyCities {
		distance := getDistance(myCity, enemyCity, state.Distances)
		if distance < minDistance && distance > 0 {
			minDistance = distance
			attackTo = enemyCity
		}
	}

	return attackTo
}

func findNearestAttack(myCities, enemyCities []string, state *api.State) (string, string) {
	minDistance := int64(math.MaxInt64)
	var attackFrom, attackTo string

	for _, myCity := range myCities {
		if !hasTroops(state.Cities[myCity]) {
			continue
		}
		for _, enemyCity := range enemyCities {
			distance := getDistance(myCity, enemyCity, state.Distances)
			if distance < minDistance && distance > 0 {
				minDistance = distance
				attackFrom = myCity
				attackTo = enemyCity
			}
		}
	}

	return attackFrom, attackTo
}

func getDistance(city1, city2 string, distances map[string]*api.Distance) int64 {
	// Try both combinations as the edge key
	if dist, exists := distances[city1+city2]; exists {
		return dist.Distance
	}
	if dist, exists := distances[city2+city1]; exists {
		return dist.Distance
	}
	return math.MaxInt64 // No connection found
}
