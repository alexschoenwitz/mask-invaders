package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	defaultAPIURL = "http://localhost:8080"
)

// the GameStateManager allows decoupling of state of turns in the game engine
// and the consuming of those turns in the UI engine
//
// All turns will be persisted in memory
// the UI egnine interacts with the GameStateManager by
// 1. accessing the current turn
// 2. moving to the next/previous turn
type GameStateManager struct {
	apiURL       string
	httpClient   *http.Client
	states       map[int]*api.State
	currentState *api.State
	currentTurn  int
	maxTurn      int

	minBufferSize   int
	stateBufferSize int // TODO(PC): should not keep the whole history in memory, for now we do
	pollInterval    time.Duration
	lastPoll        time.Time
}

func NewGameStateManager(apiURL string) (*GameStateManager, error) {
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	stateBufferSize := 25
	g := &GameStateManager{
		apiURL:     apiURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		states:     make(map[int]*api.State, stateBufferSize),

		stateBufferSize: stateBufferSize,
		minBufferSize:   5,
		pollInterval:    200 * time.Millisecond,
		lastPoll:        time.Now(),
		maxTurn:         -1,
	}

	return g, nil
}

func (g *GameStateManager) setToFirstTurn() error {
	if len(g.states) == 0 {
		return fmt.Errorf("no states available")
	}

	g.currentTurn = 0
	g.currentState = g.states[g.currentTurn]
	return nil
}

func (g *GameStateManager) hasNextTurn() bool {
	if len(g.states) > g.currentTurn {
		fmt.Println("**********************************************************************************************************************")
		fmt.Println("**********************************************moving from turn: ", g.currentTurn, "to: ", g.currentTurn+1, "***********************************************************")
		fmt.Println("**********************************************************************************************************************")
		g.currentTurn++
		g.currentState = g.states[g.currentTurn]
		return true
	}

	return false
}

func (g *GameStateManager) hasPreviousTurn() bool {
	if g.currentTurn > 0 {
		g.currentTurn--
		g.currentState = g.states[g.currentTurn]
		return true
	}

	return false
}

func (g *GameStateManager) isReadyToRenderGame() bool {
	if len(g.states) < g.minBufferSize ||
		g.currentTurn+1 == g.maxTurn { // we only render if we have at least one more turn
		return false
	}

	return true
}

func (g *GameStateManager) pollServerAndUpdateBuffer() error {
	now := time.Now()
	if now.Sub(g.lastPoll) < g.pollInterval {
		return nil
	}

	g.lastPoll = now
	return g.updateStateHistory()
}

func (g *GameStateManager) updateStateHistory() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", g.apiURL+"/v1/state:history", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch state: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	stateResponse := api.GetStateHistoryResponse{}
	if err := protojson.Unmarshal(body, &stateResponse); err != nil {
		return fmt.Errorf("failed to parse JSON: %v", err)
	}

	if stateResponse.States == nil {
		// Game hasn't started yet
		return nil
	}

	// check if the state has a new turn, if yes, store it
	if len(stateResponse.GetStates()) == len(g.states) {
		return nil
	}

	for _, t := range stateResponse.GetStates() {
		if _, ok := g.states[int(t.GetTurn())]; !ok {
			g.states[int(t.GetTurn())] = t
			g.maxTurn = int(t.GetTurn()) // is used to decide whether we can render
		}
	}

	// TODO(PC): Take care of buffer size, and paginations of history once implmented

	return nil
}
