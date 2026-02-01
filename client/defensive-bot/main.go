package main

import (
	"bytes"
	"encoding/json"
	"flag"
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
	botName   = "DefensiveBot"

	troopA = "A"
	troopB = "B"
	troopC = "C"

	// Defensive thresholds
	minDefenseReserve     = 15  // Always keep at least this many troops in each city
	minTroopsBeforeAttack = 30  // Only attack if we have this many troops total
	defenseReserveRatio   = 0.5 // Keep 50% of troops for defense
	mediumTargetMin       = 8   // Target enemies with at least this many troops
	mediumTargetMax       = 25  // Target enemies with at most this many troops
)

var (
	attemptStart = flag.Bool("s", false, "flag to try to start the game")
	gameID       = flag.String("g", "", "game id")
)

func init() {
	flag.Parse()
}

func main() {
	token, playerID, err := register(*gameID)
	if err != nil {
		log.Fatalf("Failed to register: %v", err)
	}
	log.SetPrefix("[DefensiveBot][" + playerID + "][" + botName + "] ")
	log.Printf("Registered with token: %s, playerID: %s", token, playerID)

	client := &http.Client{Timeout: 10 * time.Second}

	if *attemptStart {
		gameStarted := false
		startTime := time.Now()

		for time.Since(startTime) < 2*time.Minute {
			err := startGame(client, token, *gameID)
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
	}

	playGame(client, token, playerID, *gameID)
}

func register(gameID string) (string, string, error) {
	reqBody := &api.RegisterRequest{
		GameId: gameID,
		Name:   botName,
	}
	jsonData, err := protojson.Marshal(reqBody)
	if err != nil {
		return "", "", err
	}

	resp, err := http.Post(serverURL+"/v1/games/"+gameID+"/register", "application/json", bytes.NewBuffer(jsonData))
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

func startGame(client *http.Client, token string, gameID string) error {
	reqBody := &api.StartGameRequest{GameId: gameID}
	jsonData, err := protojson.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", serverURL+"/v1/games/"+gameID+"/start", bytes.NewBuffer(jsonData))
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
		v := make(map[string]string)
		_ = json.Unmarshal(body, &v)
		if v["message"] == "game already started" {
			return nil
		}
		return fmt.Errorf("start game failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func playGame(client *http.Client, token, playerID, gameID string) {
	log.Println("Waiting for game to start...")

	for {
		state, err := getGameState(client, token, gameID)
		if err != nil {
			log.Printf("Failed to get game state: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if state != nil && state.Turn > 0 {
			log.Printf("Game started at turn %d!", state.Turn)
			break
		}

		log.Println("Game not started yet, waiting...")
		time.Sleep(1 * time.Second)
	}

	log.Println("Starting defensive strategy game loop...")

	// Track which cities have acted this turn to prevent multiple actions per turn
	lastActionTurn := make(map[string]int64)
	lastTurn := int64(0)

	for {
		time.Sleep(10 * time.Millisecond)

		state, err := getGameState(client, token, gameID)
		if err != nil {
			log.Printf("Failed to get game state: %v", err)
			continue
		}

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

		// Only process if turn has advanced
		if state.Turn == lastTurn {
			continue
		}

		// Calculate average enemy troop strength for comparison
		avgEnemyStrength := calculateAverageEnemyStrength(enemyCities, state)

		// Check if we're behind in city count (stalemate condition)
		behindInCities := len(myCities) < len(enemyCities)
		cityRatio := float64(len(myCities)) / float64(len(myCities)+len(enemyCities))

		log.Printf("Turn %d: My cities: %d, Enemy cities: %d, Avg enemy strength: %.1f",
			state.Turn, len(myCities), len(enemyCities), avgEnemyStrength)

		lastTurn = state.Turn

		// Process each city with defensive strategy
		for _, cityName := range myCities {
			// Skip if this city already acted this turn
			if lastActionTurn[cityName] >= state.Turn {
				continue
			}

			// Mark this city as acted to prevent duplicate actions
			lastActionTurn[cityName] = state.Turn

			city := state.Cities[cityName]
			totalTroops := getTroopStrength(city.Troops)

			// Priority 1: Build up defense if below minimum reserve (unless desperately behind)
			if totalTroops < minDefenseReserve && !behindInCities {
				troopType := chooseBestTroopType(enemyCities, state)
				log.Printf("City %s: Building to minimum defense (troops: %d, target: %d)",
					cityName, totalTroops, minDefenseReserve)
				err = createTroop(client, token, playerID, gameID, cityName, troopType)
				if err != nil {
					log.Printf("Failed to create troop in %s: %v", cityName, err)
				}
				continue
			}

			// Priority 1.5: Build up to safe threshold (reduce if behind in cities)
			safeMultiplier := 1.5
			if behindInCities {
				safeMultiplier = 1.0 // Lower threshold when behind
			}
			minSafeTroops := int64(math.Max(float64(minDefenseReserve), avgEnemyStrength*safeMultiplier))
			if totalTroops < minSafeTroops && cityRatio > 0.4 {
				troopType := chooseBestTroopType(enemyCities, state)
				log.Printf("City %s: Building defense (troops: %d, target: %d)",
					cityName, totalTroops, minSafeTroops)
				err = createTroop(client, token, playerID, gameID, cityName, troopType)
				if err != nil {
					log.Printf("Failed to create troop in %s: %v", cityName, err)
				}
				continue
			}

			// Priority 2: Attack if we have sufficient troops (be more aggressive when behind)
			minAttackThreshold := minTroopsBeforeAttack
			if behindInCities {
				minAttackThreshold = minDefenseReserve + 10 // Lower threshold when behind
				log.Printf("City %s: Behind in cities, lowering attack threshold to %d", cityName, minAttackThreshold)
			}

			if totalTroops >= int64(minAttackThreshold) {
				target, attackStrength := findMediumTarget(cityName, myCities, enemyCities, state, avgEnemyStrength, behindInCities)

				if target != "" {
					// Calculate troops to send (keep defense reserve)
					reserveRatio := defenseReserveRatio
					if behindInCities {
						reserveRatio = 0.3 // Keep less when behind in cities
					}
					keepForDefense := int64(float64(totalTroops) * reserveRatio)

					// Always keep at least minDefenseReserve (unless desperately behind)
					minKeep := minDefenseReserve
					if behindInCities && cityRatio < 0.3 {
						minKeep = minDefenseReserve / 2 // Desperate measures
					}
					if keepForDefense < int64(minKeep) {
						keepForDefense = int64(minKeep)
					}

					// Adjust keep based on enemy strength (less conservative when behind)
					strengthMultiplier := 1.2
					if behindInCities {
						strengthMultiplier = 0.8
					}
					minKeepByEnemyStrength := int64(math.Max(float64(minKeep), avgEnemyStrength*strengthMultiplier))
					if keepForDefense < minKeepByEnemyStrength {
						keepForDefense = minKeepByEnemyStrength
					}

					// Ensure we keep enough for defense and send enough to win
					if totalTroops-keepForDefense >= attackStrength {
						// Send troops proportionally from available troops
						troopsToSend := make(map[string]int64)
						for troopType, count := range city.Troops {
							if count > 0 {
								sendCount := count - (count * keepForDefense / totalTroops)
								if sendCount > 0 {
									troopsToSend[troopType] = sendCount
								}
							}
						}

						sendTotal := getTroopStrength(troopsToSend)
						keepTotal := totalTroops - sendTotal

						if len(troopsToSend) > 0 && sendTotal > attackStrength {
							log.Printf("City %s: Attacking %s", cityName, target)
							log.Printf("  Before: Total=%d (A:%d B:%d C:%d)",
								totalTroops,
								city.Troops["A"], city.Troops["B"], city.Troops["C"])
							log.Printf("  Sending: Total=%d (A:%d B:%d C:%d)",
								sendTotal,
								troopsToSend["A"], troopsToSend["B"], troopsToSend["C"])
							log.Printf("  Should Keep: %d troops", keepTotal)

							err = attack(client, token, playerID, gameID, cityName, target, troopsToSend)
							if err != nil {
								log.Printf("Failed to attack from %s: %v", cityName, err)
							}
							continue
						}
					}
				}
			}

			// Priority 3: Keep building troops if no good targets (cap at lower value when behind)
			maxBuildTarget := int64(math.Max(float64(minDefenseReserve*2), avgEnemyStrength*2))
			if behindInCities {
				// Don't build past a certain point when behind, force aggression
				maxBuildTarget = int64(math.Max(float64(minDefenseReserve*1.5), avgEnemyStrength*1.2))
			}
			if totalTroops < maxBuildTarget {
				troopType := chooseBestTroopType(enemyCities, state)
				log.Printf("City %s: Building reserves (troops: %d, target: %d)", cityName, totalTroops, maxBuildTarget)
				err = createTroop(client, token, playerID, gameID, cityName, troopType)
				if err != nil {
					log.Printf("Failed to create troop in %s: %v", cityName, err)
				}
			} else if behindInCities {
				// Force attack even with large forces if behind in cities
				log.Printf("City %s: Behind in cities, forcing aggressive attack with %d troops", cityName, totalTroops)
				target, _ := findMediumTarget(cityName, myCities, enemyCities, state, avgEnemyStrength, true)
				if target != "" {
					troopsToSend := make(map[string]int64)
					keepForDefense := int64(10) // Minimal keep
					for troopType, count := range city.Troops {
						if count > 0 {
							sendCount := count - (count * keepForDefense / totalTroops)
							if sendCount > 0 {
								troopsToSend[troopType] = sendCount
							}
						}
					}
					if len(troopsToSend) > 0 {
						log.Printf("City %s: Desperate attack on %s with %d troops", cityName, target, getTroopStrength(troopsToSend))
						err = attack(client, token, playerID, gameID, cityName, target, troopsToSend)
						if err != nil {
							log.Printf("Failed to attack from %s: %v", cityName, err)
						}
					}
				}
			}
		}
	}
}

func getGameState(client *http.Client, token string, gameID string) (*api.State, error) {
	req, err := http.NewRequest("GET", serverURL+"/v1/games/"+gameID+"/state", nil)
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

func calculateAverageEnemyStrength(enemyCities []string, state *api.State) float64 {
	if len(enemyCities) == 0 {
		return 0
	}

	total := int64(0)
	for _, cityName := range enemyCities {
		total += getTroopStrength(state.Cities[cityName].Troops)
	}

	return float64(total) / float64(len(enemyCities))
}

func findMediumTarget(fromCity string, myCities, enemyCities []string, state *api.State, avgEnemyStrength float64, behindInCities bool) (string, int64) {
	type targetOption struct {
		city     string
		strength int64
		distance int64
		score    float64
	}

	var options []targetOption

	for _, enemyCity := range enemyCities {
		distance := getDistance(fromCity, enemyCity, state.Distances)
		if distance == math.MaxInt64 {
			continue
		}

		enemyStrength := getTroopStrength(state.Cities[enemyCity].Troops)

		// When behind, target any enemy city, not just medium-sized ones
		if !behindInCities {
			// Normal: target medium-sized enemies
			if enemyStrength < mediumTargetMin || enemyStrength > mediumTargetMax {
				continue
			}
		}

		// Check if we can successfully capture it
		troopsNeeded := enemyStrength + int64(avgEnemyStrength*0.5) // Enough to capture + defend

		// Score: prefer closer medium targets that we can handle
		score := 100.0 / (float64(distance+1) * (1.0 + math.Abs(float64(enemyStrength)-avgEnemyStrength)/avgEnemyStrength))

		options = append(options, targetOption{
			city:     enemyCity,
			strength: troopsNeeded,
			distance: distance,
			score:    score,
		})
	}

	if len(options) == 0 {
		// If no medium targets, look for any vulnerable target
		for _, enemyCity := range enemyCities {
			distance := getDistance(fromCity, enemyCity, state.Distances)
			if distance == math.MaxInt64 {
				continue
			}

			enemyStrength := getTroopStrength(state.Cities[enemyCity].Troops)
			myTroops := getTroopStrength(state.Cities[fromCity].Troops)

			// Only attack if we're significantly stronger
			if myTroops < enemyStrength*2 {
				continue
			}

			score := 100.0 / (float64(distance+1) * float64(enemyStrength+1))
			options = append(options, targetOption{
				city:     enemyCity,
				strength: enemyStrength + int64(avgEnemyStrength*0.3),
				distance: distance,
				score:    score,
			})
		}
	}

	// Tie-breaker: If still no targets and we have large force, attack strongest enemy
	if len(options) == 0 {
		myTroops := getTroopStrength(state.Cities[fromCity].Troops)

		// Only use tie-breaker if we have a substantial force (60+ troops)
		if myTroops >= 60 {
			for _, enemyCity := range enemyCities {
				distance := getDistance(fromCity, enemyCity, state.Distances)
				if distance == math.MaxInt64 {
					continue
				}

				enemyStrength := getTroopStrength(state.Cities[enemyCity].Troops)

				// Attack if we have at least 1.5x their strength
				if myTroops >= enemyStrength*3/2 {
					score := 100.0 / float64(distance+1)
					options = append(options, targetOption{
						city:     enemyCity,
						strength: enemyStrength + int64(avgEnemyStrength*0.5),
						distance: distance,
						score:    score,
					})
				}
			}
		}
	}

	if len(options) == 0 {
		return "", 0
	}

	// Pick best scored target
	best := options[0]
	for _, opt := range options[1:] {
		if opt.score > best.score {
			best = opt
		}
	}

	return best.city, best.strength
}

func getTroopStrength(troops map[string]int64) int64 {
	total := int64(0)
	for _, count := range troops {
		total += count
	}
	return total
}

func getDistance(city1, city2 string, distances map[string]*api.Distance) int64 {
	if dist, exists := distances[city1+city2]; exists {
		return dist.Distance
	}
	if dist, exists := distances[city2+city1]; exists {
		return dist.Distance
	}
	return math.MaxInt64
}

func chooseBestTroopType(enemyCities []string, state *api.State) string {
	enemyTroopCounts := map[string]int64{
		troopA: 0,
		troopB: 0,
		troopC: 0,
	}

	for _, cityName := range enemyCities {
		city := state.Cities[cityName]
		for troopType, count := range city.Troops {
			enemyTroopCounts[troopType] += count
		}
	}

	// Choose counter: A beats C, B beats A, C beats B
	if enemyTroopCounts[troopC] > enemyTroopCounts[troopA] && enemyTroopCounts[troopC] > enemyTroopCounts[troopB] {
		return troopA
	} else if enemyTroopCounts[troopA] > enemyTroopCounts[troopB] {
		return troopB
	} else {
		return troopC
	}
}

func createTroop(client *http.Client, token, playerID, gameID, city, troopType string) error {
	action := &api.PostActionRequest{
		GameId: gameID,
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

	req, err := http.NewRequest("POST", serverURL+"/v1/games/"+gameID+"/action", bytes.NewBuffer(jsonData))
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

func attack(client *http.Client, token, playerID, gameID, from, to string, troops map[string]int64) error {
	action := &api.PostActionRequest{
		GameId: gameID,
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
		return fmt.Errorf("no troops to attack with")
	}

	jsonData, err := protojson.Marshal(action)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", serverURL+"/v1/games/"+gameID+"/action", bytes.NewBuffer(jsonData))
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
