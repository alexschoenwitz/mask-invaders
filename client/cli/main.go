package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"github.com/alexschoenwitz/mask-invaders/client"
)

const (
	helpMessage = `Mask Invaders CLI
Usage:
	register <username>
	getstate
	attack <from_city_id> <attack_type> <to_city_id> <A:x,B:y,C:z>
	noop
`
)

func main() {
	c := client.NewClient("http://localhost:8080")
	scanner := bufio.NewScanner(os.Stdin)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var playerID string

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// read stdin input until newline
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()

		// process input
		// e.g. "register", "getstate", "attack", "train", "claimMine" etc.

		parts := strings.Split(input, " ")
		if len(parts) < 1 || strings.TrimSpace(parts[0]) == "" {
			fmt.Println("Expected input of the form: <command> [args...]")
			fmt.Print(helpMessage)
			continue
		}
		switch parts[0] {
		case "register":
			if len(parts) < 2 {
				fmt.Println("Expected input of the form: register <username>")
				continue
			}
			var err error
			playerID, err = c.Register(parts[1])
			if err != nil {
				panic(err)
			}
			fmt.Printf("Registered with player ID: %s\n", playerID)
		case "getstate":
			state, err := c.GetState()
			if err != nil {
				panic(err)
			}
			// pretty print state in formated json
			b, _ := json.MarshalIndent(state, "", "    ")
			fmt.Printf("Current State:\n%s\n", string(b))
		case "startgame":
			err := c.StartGame()
			if err != nil {
				panic(err)
			}
			fmt.Println("Game started.")
		case "attack":
			if len(parts) < 4 {
				fmt.Println("Expected input of the form: attack <from_city_id> <to_city_id> <A:x,B:y,C:z>")
				continue
			}
			fromCityID := parts[1]
			attackType := parts[2]
			toID := parts[3]
			troopStr := parts[4]
			troops := make(map[string]int64)
			troopParts := strings.Split(troopStr, ",")
			for _, tp := range troopParts {
				kv := strings.Split(tp, ":")
				if len(kv) != 2 {
					fmt.Printf("Invalid troop format: %s\n", tp)
					continue
				}
				count, err := strconv.ParseInt(kv[1], 10, 64)
				if err != nil {
					fmt.Printf("Invalid troop count: %s\n", kv[1])
					continue
				}
				troops[kv[0]] = count
			}
			switch attackType {
			case "city":
				req := &api.PostActionRequest{
					Action: &api.Action{
						Player: playerID,
						Action: &api.Action_Attack{
							Attack: &api.Attack{
								From:   fromCityID,
								To:     &api.Attack_City{City: toID},
								Troops: troops,
							},
						},
					},
				}
				err := c.PostAction(req)
				if err != nil {
					panic(err)
				}
				fmt.Println("Attack action sent.")
			case "mine":
				req := &api.PostActionRequest{
					Action: &api.Action{
						Player: playerID,
						Action: &api.Action_Attack{
							Attack: &api.Attack{
								From:   fromCityID,
								To:     &api.Attack_Mine{Mine: toID},
								Troops: troops,
							},
						},
					},
				}
				err := c.PostAction(req)
				if err != nil {
					panic(err)
				}
				fmt.Println("Mine attack action sent.")
			default:
				fmt.Printf("Unknown attack type: %s\n", attackType)
			}
		case "noop":
			req := &api.PostActionRequest{
				Action: &api.Action{
					Player: playerID,
					Action: &api.Action_None{},
				},
			}
			err := c.PostAction(req)
			if err != nil {
				panic(err)
			}
			fmt.Println("Noop action sent.")
		case "help":
			fmt.Print(helpMessage)
		case "exit":
			fmt.Println("Exiting.")
			return
		default:
			fmt.Printf("Unknown command: %s\n", parts[0])
			fmt.Print(helpMessage)
		}
	}
}
