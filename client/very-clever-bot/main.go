package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	serverURL = "http://localhost:8080"
	botName   = "VeryCleverBot"

	troopA = "A"
	troopB = "B"
	troopC = "C"
)

func main() {
	// 1. Register the bot
	token, playerID, err := register()
	if err != nil {
		log.Fatalf("Failed to register: %v", err)
	}
	log.Printf("Registered with token: %s, playerID: %s", token, playerID)

	// 2. Try to start the game for 2 minutes
	client := &http.Client{Timeout: 10 * time.Second}
	gameStarted := false
	startTime := time.Now()

	for time.Since(startTime) < 2*time.Minute {
		err := startGame(client, token)
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

	// 3. Play the game with the clever strategy
	playGame(client, token, playerID)
}

func register() (string, string, error) {
	reqBody := &api.RegisterRequest{Name: botName}
	jsonData, err := protojson.Marshal(reqBody)
	if err != nil {
		return "", "", err
	}

	resp, err := http.Post(serverURL+"/v1/register", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	var regResp api.RegisterResponse
	if err := protojson.Unmarshal(body, &regResp); err != nil {
		return "", "", err
	}

	return regResp.Token, regResp.Id, nil
}

func startGame(client *http.Client, token string) error {
	req, err := http.NewRequest("POST", serverURL+"/v1/start", nil)
	if err != nil {
		return err
	}
	req.Header.Set("authorization", token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		v := make(map[string]string)
		_ = json.Unmarshal(body, &v)
		if v["message"] == "game already started" {
			return nil // good!
		}
		return fmt.Errorf("start game failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func playGame(client *http.Client, token, playerID string) {
	log.Println("Starting game loop with clever strategy...")

	for {
		time.Sleep(100 * time.Millisecond)

		state, err := getGameState(client, token)
		if err != nil {
			log.Printf("Failed to get game state: %v", err)
			continue
		}

		// Analyze the battlefield
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

		// IMPROVED STRATEGY: Concentrate forces on one target
		// 1. Find the best target to attack
		bestTarget := findWeakestTarget(myCities, enemyCities, state)

		if bestTarget == "" {
			// No valid target, build troops everywhere
			for _, cityName := range myCities {
				troopType := chooseBestTroopType(enemyCities, state)
				log.Printf("City %s: No target, building troop type %s", cityName, troopType)
				err = createTroop(client, token, playerID, cityName, troopType)
				if err != nil {
					log.Printf("Failed to create troop in %s: %v", cityName, err)
				}
			}
			continue
		}

		// 2. Coordinate attacks on the best target
		for _, cityName := range myCities {
			city := state.Cities[cityName]
			totalTroops := getTroopStrength(city.Troops)

			// Build troops if weak (less than 8 troops)
			if totalTroops < 8 {
				troopType := chooseBestTroopType(enemyCities, state)
				log.Printf("City %s: Building troop type %s (current troops: %d)", cityName, troopType, totalTroops)
				err = createTroop(client, token, playerID, cityName, troopType)
				if err != nil {
					log.Printf("Failed to create troop in %s: %v", cityName, err)
				}
			} else {
				// Attack the target, but keep 40% troops for defense
				distance := getDistance(cityName, bestTarget, state.Distances)
				if distance == math.MaxInt64 {
					// Can't reach target, build troops
					troopType := chooseBestTroopType(enemyCities, state)
					err = createTroop(client, token, playerID, cityName, troopType)
					if err != nil {
						log.Printf("Failed to create troop in %s: %v", cityName, err)
					}
					continue
				}

				// Send 60% of troops, keep 40% for defense
				attackTroops := make(map[string]int64)
				for troopType, count := range city.Troops {
					if count > 0 {
						sendCount := (count * 6) / 10
						if sendCount > 0 {
							attackTroops[troopType] = sendCount
						}
					}
				}

				if len(attackTroops) > 0 {
					log.Printf("City %s: Attacking %s with %d troops (keeping 40%% defense)", cityName, bestTarget, getTroopStrength(attackTroops))
					err = attack(client, token, playerID, cityName, bestTarget, attackTroops)
					if err != nil {
						log.Printf("Failed to attack from %s: %v", cityName, err)
					}
				}
			}
		}
	}
}

func getGameState(client *http.Client, token string) (*api.State, error) {
	req, err := http.NewRequest("GET", serverURL+"/v1/state", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get state failed with status %d: %s", resp.StatusCode, string(body))
	}

	var stateResp api.GetStateResponse
	if err := protojson.Unmarshal(body, &stateResp); err != nil {
		return nil, err
	}

	return stateResp.State, nil
}

// Find the weakest enemy target that we can all attack
func findWeakestTarget(myCities, enemyCities []string, state *api.State) string {
	if len(enemyCities) == 0 {
		return ""
	}

	type targetInfo struct {
		city     string
		strength int64
		distance int64
	}

	var targets []targetInfo

	// Evaluate each enemy city
	for _, enemyCity := range enemyCities {
		enemyStrength := getTroopStrength(state.Cities[enemyCity].Troops)

		// Find closest distance from any of our cities
		minDistance := int64(math.MaxInt64)
		for _, myCity := range myCities {
			dist := getDistance(myCity, enemyCity, state.Distances)
			if dist < minDistance {
				minDistance = dist
			}
		}

		if minDistance < math.MaxInt64 {
			targets = append(targets, targetInfo{
				city:     enemyCity,
				strength: enemyStrength,
				distance: minDistance,
			})
		}
	}

	if len(targets) == 0 {
		return ""
	}

	// Choose the weakest target that's reasonably close
	bestScore := float64(-math.MaxInt64)
	bestTarget := ""

	for _, target := range targets {
		// Prefer weak targets that are close
		score := 100.0 / (float64(target.strength+1) * math.Sqrt(float64(target.distance+1)))
		if score > bestScore {
			bestScore = score
			bestTarget = target.city
		}
	}

	return bestTarget
}

// Find the best attack target for a specific city
func findBestAttackForCity(fromCity string, enemyCities []string, state *api.State, playerID string) (string, map[string]int64) {
	myCity := state.Cities[fromCity]
	myStrength := getTroopStrength(myCity.Troops)
	if myStrength == 0 {
		return "", nil
	}

	type attackOption struct {
		to       string
		weakness int64
		distance int64
	}

	var options []attackOption

	for _, enemyCity := range enemyCities {
		distance := getDistance(fromCity, enemyCity, state.Distances)
		if distance == math.MaxInt64 {
			continue
		}

		enemyStrength := getTroopStrength(state.Cities[enemyCity].Troops)

		options = append(options, attackOption{
			to:       enemyCity,
			weakness: enemyStrength,
			distance: distance,
		})
	}

	if len(options) == 0 {
		return "", nil
	}

	// Select the best target: weak enemies that are close
	bestScore := float64(-math.MaxInt64)
	var bestTarget string

	for _, opt := range options {
		// Prefer weak enemies that are close
		score := float64(myStrength) / (float64(opt.weakness+1) * math.Sqrt(float64(opt.distance+1)))

		if score > bestScore {
			bestScore = score
			bestTarget = opt.to
		}
	}

	if bestTarget == "" {
		return "", nil
	}

	// Return all troops for this attack
	validTroops := make(map[string]int64)
	for troopType, count := range myCity.Troops {
		if count > 0 {
			validTroops[troopType] = count
		}
	}

	return bestTarget, validTroops
}

// Calculate total troop strength for a city
func getTroopStrength(troops map[string]int64) int64 {
	total := int64(0)
	for _, count := range troops {
		total += count
	}
	return total
}

// Find the optimal attack: strongest city attacks weakest enemy within range
func findOptimalAttack(myCities, enemyCities []string, state *api.State, playerID string) (string, string, map[string]int64) {
	type attackOption struct {
		from     string
		to       string
		strength int64
		weakness int64
		distance int64
		troops   map[string]int64
	}

	var options []attackOption

	for _, myCity := range myCities {
		myStrength := getTroopStrength(state.Cities[myCity].Troops)
		if myStrength == 0 {
			continue
		}

		for _, enemyCity := range enemyCities {
			distance := getDistance(myCity, enemyCity, state.Distances)
			if distance == math.MaxInt64 {
				continue
			}

			enemyStrength := getTroopStrength(state.Cities[enemyCity].Troops)

			// Filter out troops with 0 count
			validTroops := make(map[string]int64)
			for troopType, count := range state.Cities[myCity].Troops {
				if count > 0 {
					validTroops[troopType] = count
				}
			}

			// Score: prioritize weak enemies, close distance, and our strong cities
			options = append(options, attackOption{
				from:     myCity,
				to:       enemyCity,
				strength: myStrength,
				weakness: enemyStrength,
				distance: distance,
				troops:   validTroops,
			})
		}
	}

	if len(options) == 0 {
		return "", "", nil
	}

	// Select the best attack: target weak enemies that are close with our strong cities
	// Score formula: prefer weak enemies / short distance, scaled by our strength
	bestScore := float64(-math.MaxInt64)
	var bestOption attackOption

	for _, opt := range options {
		// Higher score = better target
		// We want: low enemy strength, low distance, high our strength
		score := float64(opt.strength) / (float64(opt.weakness+1) * math.Sqrt(float64(opt.distance+1)))

		if score > bestScore {
			bestScore = score
			bestOption = opt
		}
	}

	log.Printf("Clever attack: %s (strength: %d) -> %s (weakness: %d) distance: %d, score: %.2f",
		bestOption.from, bestOption.strength, bestOption.to, bestOption.weakness, bestOption.distance, bestScore)

	return bestOption.from, bestOption.to, bestOption.troops
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

// Determine if we should build troops and where/what
func shouldBuildTroops(myCities, enemyCities []string, state *api.State) (bool, string, string) {
	// Build troops in cities that have low troop counts
	const minTroopsPerCity = 5

	for _, cityName := range myCities {
		city := state.Cities[cityName]
		totalTroops := getTroopStrength(city.Troops)

		if totalTroops < minTroopsPerCity {
			// Analyze enemy composition to choose best counter troop
			troopType := chooseBestTroopType(enemyCities, state)
			log.Printf("Building troop type %s in city %s (current troops: %d)", troopType, cityName, totalTroops)
			return true, cityName, troopType
		}
	}

	return false, "", ""
}

// Choose the best troop type based on enemy composition
func chooseBestTroopType(enemyCities []string, state *api.State) string {
	enemyTroopCounts := map[string]int64{
		troopA: 0,
		troopB: 0,
		troopC: 0,
	}

	// Count enemy troop types
	for _, cityName := range enemyCities {
		city := state.Cities[cityName]
		for troopType, count := range city.Troops {
			enemyTroopCounts[troopType] += count
		}
	}

	// Choose counter based on rock-paper-scissors logic
	// A is strong vs C (2.1), B is strong vs A (1.3), C is strong vs B (1.1)
	if enemyTroopCounts[troopC] > enemyTroopCounts[troopA] && enemyTroopCounts[troopC] > enemyTroopCounts[troopB] {
		return troopA // Counter C with A
	} else if enemyTroopCounts[troopA] > enemyTroopCounts[troopB] {
		return troopB // Counter A with B
	} else {
		return troopC // Counter B with C
	}
}

func createTroop(client *http.Client, token, playerID, city, troopType string) error {
	action := &api.PostActionRequest{
		Action: &api.Action{
			Player: playerID,
			Action: &api.Action_CreateTroop{
				CreateTroop: &api.CreateTroop{
					In:   city,
					Type: troopType,
				},
			},
		},
	}

	jsonData, err := protojson.Marshal(action)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", serverURL+"/v1/action", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authorization", token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create troop failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func attack(client *http.Client, token, playerID, from, to string, troops map[string]int64) error {
	action := &api.PostActionRequest{
		Action: &api.Action{
			Player: playerID,
			Action: &api.Action_Attack{
				Attack: &api.Attack{
					From:   from,
					To:     &api.Attack_City{City: to},
					Troops: troops,
				},
			},
		},
	}

	noTroops := true
	for _, amount := range troops {
		if amount > 0 {
			noTroops = false
			break
		}
	}
	if noTroops {
		log.Println("No troops to attack with")
		action.Action.Action = &api.Action_None{}
	} else {
		log.Printf("Executing attack from %s to %s with %d total troops", from, to, getTroopStrength(troops))
	}

	jsonData, err := protojson.Marshal(action)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", serverURL+"/v1/action", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authorization", token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("attack failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
