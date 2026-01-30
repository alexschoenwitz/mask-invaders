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
	botName   = "DumbBot"
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

	// 3. Play the game with the dumb strategy
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
	log.Println("Starting game loop...")

	for {
		time.Sleep(1 * time.Second) // Give some time between turns

		state, err := getGameState(client, token)
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

		// Find the nearest enemy city to attack
		attackFrom, attackTo := findNearestAttack(myCities, enemyCities, state)
		if attackFrom == "" || attackTo == "" {
			log.Println("No valid attack found")
			continue
		}

		// Attack with all troops from the attacking city
		city := state.Cities[attackFrom]
		if len(city.Troops) == 0 {
			log.Printf("No troops in city %s", attackFrom)
			continue
		}

		err = attack(client, token, playerID, attackFrom, attackTo, city.Troops)
		if err != nil {
			log.Printf("Failed to attack: %v", err)
			continue
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

func findNearestAttack(myCities, enemyCities []string, state *api.State) (string, string) {
	minDistance := int64(math.MaxInt64)
	var attackFrom, attackTo string

	for _, myCity := range myCities {
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

func attack(client *http.Client, token, playerID, from, to string, troops map[string]int64) error {
	action := &api.PostActionRequest{
		Action: &api.Action{
			Player: playerID,
			Action: &api.Action_Attack{
				Attack: &api.Attack{
					From:   from,
					To:     to,
					Troops: troops,
				},
			},
		},
	}

	noTroops := true
	for _, ammount := range troops {
		if ammount > 0 {
			noTroops = false
			break
		}
	}
	if noTroops {
		fmt.Println("No troops to attack with")
		action.Action.Action = &api.Action_None{}
	} else {
		log.Printf("Will attack from %s to %s with troops %+v", from, to, troops)
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
