package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"github.com/alexschoenwitz/mask-invaders/client"
	"github.com/alexschoenwitz/mask-invaders/client/advanced-bot/strategy"
)

func main() {
	serverURL := flag.String("server", "http://localhost:8080", "Server URL")
	gameID := flag.String("g", "", "Game ID to join")
	botName := flag.String("name", "AdvancedBot", "Bot name")
	flag.Parse()

	if *gameID == "" {
		log.Fatal("Game ID is required (use -game flag)")
	}

	// Create client and register
	c := client.NewClient(*serverURL)
	playerID, err := c.Register(*botName, *gameID)
	if err != nil {
		log.Fatalf("Failed to register: %v", err)
	}
	log.SetPrefix("[AdvancedBot][" + playerID + "][" + *botName + "] ")
	log.Printf("Registered as player %s with name %s", playerID, *botName)

	// Create bot instance
	bot := &Bot{
		client:   c,
		gameID:   *gameID,
		playerID: playerID,
		strategy: strategy.NewStrategy(playerID),
	}

	// Run the game loop
	ctx := context.Background()
	bot.Run(ctx)
}

type Bot struct {
	client   *client.Client
	gameID   string
	playerID string
	strategy *strategy.Strategy
}

func (b *Bot) Run(ctx context.Context) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	log.Println("Bot started, waiting for game state...")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := b.playTurn(); err != nil {
				log.Printf("Error playing turn: %v", err)
			}
		}
	}
}

func (b *Bot) playTurn() error {
	// Get current game state
	state, err := b.client.GetState(b.gameID)
	if err != nil {
		return fmt.Errorf("failed to get state: %w", err)
	}

	if state == nil {
		return nil // Game not started yet
	}

	// Check if we still have cities
	hasCities := false
	for _, city := range state.Cities {
		if city.Player == b.playerID {
			hasCities = true
			break
		}
	}

	if !hasCities {
		log.Println("We lost all cities. Game over.")
		return nil
	}

	// Decide actions for this turn
	startTime := time.Now()
	actions := b.strategy.DecideActions(state)
	decisionTime := time.Since(startTime)

	log.Printf("Turn %d: Generated %d actions in %v", state.Turn, len(actions), decisionTime)

	// Submit actions
	for _, action := range actions {
		action.Player = b.playerID
		req := &api.PostActionRequest{
			GameId: b.gameID,
			Action: action,
		}
		if err := b.client.PostAction(req); err != nil {
			log.Printf("Failed to post action: %v", err)
		}
	}

	return nil
}
