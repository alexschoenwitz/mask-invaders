package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"time"

	_ "image/png"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
)

const (
	screenWidth      = 800
	screenHeight     = 800
	minCitySize      = 50
	maxCitySize      = 60
	pollInterval     = 200 * time.Millisecond // How often to poll the server
	turnPlaybackRate = 500 * time.Millisecond // Fixed rate to consume states from buffer
	minBufferStates  = 3                      // Minimum states to buffer before starting playback
	stateBufferSize  = 20                     // Number of states to keep in buffer
	defaultAPIURL    = "http://localhost:8080"
)

type Troops map[string]int64

type CityDisplay struct {
	Name   string
	Player string
	Troops Troops
	X, Y   float64
	Size   float64
}

type MovementDisplay struct {
	From         *CityDisplay
	To           *CityDisplay
	Troops       Troops
	Player       string
	StartTurn    int64
	ArrivingTurn int64
	Progress     float64 // 0.0 to 1.0
}

// StateBuffer holds a state and when it was received
type StateBuffer struct {
	state      *api.State
	receivedAt time.Time
}

// Game represents the main game state
type Game struct {
	apiURL           string
	httpClient       *http.Client
	states           []*api.State  // All states (for replay mode)
	stateBuffer      []StateBuffer // Buffered states with timestamps (for live mode)
	currentStateIdx  int
	displayStateIdx  int // Index in buffer we're currently displaying
	lastUpdate       time.Time
	lastPoll         time.Time
	turnProgress     float64 // 0.0 to 1.0 progress within current turn
	currentTurn      float64 // Continuous current turn with sub-turn precision
	cities           map[string]*CityDisplay
	movements        []*MovementDisplay
	playerColors     map[string]color.RGBA
	colorPalette     []color.RGBA
	playerList       []string
	offscreen        *ebiten.Image
	isLiveMode       bool             // true if using live API, false if replay mode
	animationStart   time.Time        // When we started animating current turn
	turnDuration     time.Duration    // Duration for current turn animation
	screenWidth      int              // Current screen width
	screenHeight     int              // Current screen height
	movementStartMap map[string]int64 // Cache of movement ID -> start turn
	tickCounter      int              // Frame counter for sprite animations
	// Game selection
	gameID           string   // Currently selected game ID
	availableGames   []string // List of available game IDs
	dropdownExpanded bool     // Whether the dropdown is expanded
	dropdownY        int      // Y position of the dropdown
}

// Player colors palette - softer, less saturated colors
var colors = []color.RGBA{
	{200, 80, 80, 255},   // Soft Red
	{80, 180, 80, 255},   // Soft Green
	{80, 120, 200, 255},  // Soft Blue
	{220, 200, 80, 255},  // Soft Yellow
	{200, 100, 200, 255}, // Soft Magenta
	{80, 200, 200, 255},  // Soft Cyan
	{220, 140, 80, 255},  // Soft Orange
	{150, 100, 200, 255}, // Soft Purple
	{220, 160, 180, 255}, // Soft Pink
	{100, 150, 100, 255}, // Soft Dark Green
	{150, 150, 150, 255}, // Soft Gray
	{220, 220, 220, 255}, // Soft White
	{150, 80, 80, 255},   // Soft Maroon
	{80, 150, 150, 255},  // Soft Teal
	{128, 128, 0, 255},   // Olive
	{0, 0, 128, 255},     // Navy
}

func NewGame(filename string) (*Game, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var states []*api.State
	if err := json.Unmarshal(data, &states); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	game := &Game{
		states:           states,
		lastUpdate:       time.Now(),
		cities:           make(map[string]*CityDisplay),
		playerColors:     make(map[string]color.RGBA),
		colorPalette:     colors,
		offscreen:        ebiten.NewImage(screenWidth, screenHeight),
		isLiveMode:       false,
		screenWidth:      screenWidth,
		screenHeight:     screenHeight,
		movementStartMap: make(map[string]int64),
	}

	game.initializeCities()
	game.assignPlayerColors()
	game.updateDisplayState() // Initialize the display state

	return game, nil
}

func NewGameLive(apiURL string) (*Game, error) {
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	game := &Game{
		apiURL:           apiURL,
		httpClient:       &http.Client{Timeout: 5 * time.Second},
		states:           []*api.State{},
		stateBuffer:      make([]StateBuffer, 0, stateBufferSize),
		lastUpdate:       time.Now(),
		lastPoll:         time.Now(),
		cities:           make(map[string]*CityDisplay),
		playerColors:     make(map[string]color.RGBA),
		colorPalette:     colors,
		offscreen:        ebiten.NewImage(screenWidth, screenHeight),
		isLiveMode:       true,
		displayStateIdx:  -1,
		screenWidth:      screenWidth,
		screenHeight:     screenHeight,
		movementStartMap: make(map[string]int64),
		availableGames:   []string{},
		dropdownExpanded: false,
		dropdownY:        20,
	}

	// Fetch list of games first
	if err := game.fetchGameList(); err != nil {
		log.Printf("Warning: Failed to fetch game list: %v", err)
	}

	// Select the first game if available
	if len(game.availableGames) > 0 {
		game.gameID = game.availableGames[0]
		log.Printf("Selected default game: %s", game.gameID)

		// Fetch historical data first for the graph
		if err := game.fetchStateHistory(); err != nil {
			log.Printf("Warning: Failed to fetch state history: %v", err)
		}

		// Try to fetch initial state
		if err := game.pollServerState(); err != nil {
			log.Printf("Warning: Failed to fetch initial state: %v", err)
		}
	} else {
		log.Printf("No games available yet")
	}

	return game, nil
}

func (g *Game) fetchStateHistory() error {
	if g.gameID == "" {
		return fmt.Errorf("no game selected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/v1/games/%s/state:history", g.apiURL, g.gameID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch history: %v", err)
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

	historyResponse := api.GetStateHistoryResponse{}
	if err := protojson.Unmarshal(body, &historyResponse); err != nil {
		return fmt.Errorf("failed to parse JSON: %v", err)
	}

	// Store all historical states
	g.states = historyResponse.States

	// Initialize cities and colors from history if we have data
	if len(g.states) > 0 {
		g.initializeCities()
		g.assignPlayerColors()
		log.Printf("Fetched %d historical states for graphing", len(g.states))
	}

	return nil
}

func (g *Game) fetchGameList() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", g.apiURL+"/v1/games", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch games: %v", err)
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

	gamesResponse := api.ListGamesResponse{}
	if err := protojson.Unmarshal(body, &gamesResponse); err != nil {
		return fmt.Errorf("failed to parse JSON: %v", err)
	}

	g.availableGames = gamesResponse.GameIds
	log.Printf("Fetched %d games", len(g.availableGames))

	return nil
}

func (g *Game) pollServerState() error {
	if g.gameID == "" {
		return nil // silently return if no game selected
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/v1/games/%s/state", g.apiURL, g.gameID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

	stateResponse := api.GetStateResponse{}
	if err := protojson.Unmarshal(body, &stateResponse); err != nil {
		return fmt.Errorf("failed to parse JSON: %v", err)
	}

	if stateResponse.State == nil {
		// Game hasn't started yet
		return nil
	}

	// Detect if a new game has started (turn number went backwards or reset)
	if len(g.stateBuffer) > 0 && stateResponse.State.Turn < g.stateBuffer[len(g.stateBuffer)-1].state.Turn {
		log.Printf("New game detected (turn %d < previous %d), resetting state", stateResponse.State.Turn, g.stateBuffer[len(g.stateBuffer)-1].state.Turn)
		// Reset everything for the new game
		g.states = []*api.State{}
		g.stateBuffer = make([]StateBuffer, 0, stateBufferSize)
		g.cities = make(map[string]*CityDisplay)
		g.playerColors = make(map[string]color.RGBA)
		g.playerList = nil
		g.movements = nil
		g.currentStateIdx = 0
		g.displayStateIdx = -1
		g.currentTurn = 0
		g.animationStart = time.Time{}
		g.movementStartMap = make(map[string]int64)
	}

	// Check if this is a new state
	isNewState := len(g.stateBuffer) == 0 || g.stateBuffer[len(g.stateBuffer)-1].state.Turn != stateResponse.State.Turn

	if isNewState {
		// Add to buffer with timestamp
		newBuffer := StateBuffer{
			state:      stateResponse.State,
			receivedAt: time.Now(),
		}
		g.stateBuffer = append(g.stateBuffer, newBuffer)

		// Keep buffer size limited
		if len(g.stateBuffer) > stateBufferSize {
			g.stateBuffer = g.stateBuffer[1:]
			if g.displayStateIdx > 0 {
				g.displayStateIdx--
			}
		}

		// Also add to states list for graph history
		// Don't limit states list - keep full history for graphing
		g.states = append(g.states, stateResponse.State)

		// Initialize cities on first state
		if len(g.cities) == 0 {
			g.initializeCities()
			g.assignPlayerColors()
		}

		log.Printf("Buffered state for turn %d, buffer size: %d", stateResponse.State.Turn, len(g.stateBuffer))
	}

	return nil
}

func (g *Game) initializeCities() {
	if len(g.states) == 0 {
		return
	}

	// Get all unique cities from the first state
	cities := g.states[0].Cities
	numCities := len(cities)
	if numCities == 0 {
		return
	}

	// Arrange cities in a circle or grid using logical coordinates (not dynamic screen size)
	centerX, centerY := float64(screenWidth)/2, float64(screenHeight)/2
	radius := float64(min(screenWidth, screenHeight)) * 0.3

	i := 0
	for name := range cities {
		var x, y float64
		if numCities == 1 {
			x, y = centerX, centerY
		} else {
			angle := 2 * math.Pi * float64(i) / float64(numCities)
			x = centerX + radius*math.Cos(angle)
			y = centerY + radius*math.Sin(angle)
		}

		g.cities[name] = &CityDisplay{
			Name:   name,
			Player: cities[name].Player,
			Troops: cities[name].Troops,
			X:      x,
			Y:      y,
		}
		i++
	}
}

func (g *Game) assignPlayerColors() {
	playerSet := make(map[string]bool)

	// Collect all unique players
	for _, state := range g.states {
		for _, city := range state.Cities {
			playerSet[city.Player] = true
		}
		for _, movement := range state.Movements {
			playerSet[movement.Player] = true
		}
	}

	// Convert to slice for consistent ordering
	for player := range playerSet {
		g.playerList = append(g.playerList, player)
	}

	// Assign colors
	for i, player := range g.playerList {
		colorIdx := i % len(g.colorPalette)
		g.playerColors[player] = g.colorPalette[colorIdx]
	}
}

func (g *Game) Update() error {
	g.tickCounter++ // Increment tick counter for sprite animations
	now := time.Now()

	// Handle mouse input for dropdown in live mode
	if g.isLiveMode {
		g.handleDropdownInput()
	}

	// Poll server in live mode
	if g.isLiveMode && now.Sub(g.lastPoll) >= pollInterval {
		g.lastPoll = now
		if err := g.pollServerState(); err != nil {
			log.Printf("Failed to poll server: %v", err)
		}
	}

	if !g.isLiveMode {
		// Replay mode: advance through states automatically
		elapsed := now.Sub(g.lastUpdate)
		g.turnProgress = float64(elapsed) / float64(turnPlaybackRate)

		// Update continuous current turn for smooth movement
		if len(g.states) > 0 && g.currentStateIdx < len(g.states) {
			baseTurn := float64(g.states[g.currentStateIdx].Turn)
			g.currentTurn = baseTurn + g.turnProgress
		}

		if elapsed >= turnPlaybackRate {
			g.lastUpdate = now
			g.turnProgress = 0.0
			g.currentStateIdx++
			if g.currentStateIdx >= len(g.states) {
				g.currentStateIdx = len(g.states) - 1
			}
			g.updateDisplayState()
		}
	} else {
		// Live mode: use delayed state buffer
		g.updateLiveMode(now)
	}

	// Always update movements every frame for smooth animation
	g.updateMovements()

	return nil
}

func (g *Game) handleDropdownInput() {
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		
		// Check if clicking on dropdown header
		dropdownX := 10
		dropdownWidth := 200
		dropdownHeight := 25
		
		if mx >= dropdownX && mx <= dropdownX+dropdownWidth &&
			my >= g.dropdownY && my <= g.dropdownY+dropdownHeight {
			// Toggle dropdown on click
			if !g.dropdownExpanded {
				g.dropdownExpanded = true
				// Fetch game list when opening dropdown
				go func() {
					if err := g.fetchGameList(); err != nil {
						log.Printf("Failed to fetch game list: %v", err)
					}
				}()
			}
			return
		}
		
		// Check if clicking on dropdown options
		if g.dropdownExpanded {
			optionHeight := 20
			for i, gameID := range g.availableGames {
				optionY := g.dropdownY + dropdownHeight + i*optionHeight
				if mx >= dropdownX && mx <= dropdownX+dropdownWidth &&
					my >= optionY && my <= optionY+optionHeight {
					// Select this game
					if g.gameID != gameID {
						g.gameID = gameID
						log.Printf("Selected game: %s", gameID)
						// Reset state for new game
						g.states = []*api.State{}
						g.stateBuffer = make([]StateBuffer, 0, stateBufferSize)
						g.cities = make(map[string]*CityDisplay)
						g.playerColors = make(map[string]color.RGBA)
						g.playerList = nil
						g.movements = nil
						g.currentStateIdx = 0
						g.displayStateIdx = -1
						g.currentTurn = 0
						g.animationStart = time.Time{}
						g.movementStartMap = make(map[string]int64)
						// Fetch new game data
						go func() {
							if err := g.fetchStateHistory(); err != nil {
								log.Printf("Failed to fetch history for game %s: %v", gameID, err)
							}
						}()
					}
					g.dropdownExpanded = false
					return
				}
			}
			// Clicked outside, close dropdown
			g.dropdownExpanded = false
		}
	}
}

func (g *Game) updateLiveMode(now time.Time) {
	// Wait until we have minimum number of states buffered
	if len(g.stateBuffer) < minBufferStates {
		return
	}

	// Start playback if not already started
	if g.displayStateIdx < 0 {
		g.displayStateIdx = 0
		g.currentStateIdx = 0
		g.animationStart = now
		g.currentTurn = float64(g.stateBuffer[0].state.Turn)
		g.updateDisplayState()
		log.Printf("Started playback from turn %d with %d states buffered",
			g.stateBuffer[0].state.Turn, len(g.stateBuffer))
		return
	}

	// Calculate interpolation progress between current and next state
	elapsed := now.Sub(g.animationStart)

	// Check if we should move to next state
	if elapsed >= turnPlaybackRate {
		// Move to next state pair
		nextStateIdx := g.displayStateIdx + 1

		if nextStateIdx < len(g.stateBuffer) {
			g.displayStateIdx = nextStateIdx
			g.currentStateIdx = nextStateIdx
			g.animationStart = g.animationStart.Add(turnPlaybackRate)
			g.updateDisplayState()

			statesAhead := len(g.stateBuffer) - g.displayStateIdx - 1
			log.Printf("Advanced to turn %d, %d states buffered ahead",
				g.stateBuffer[nextStateIdx].state.Turn, statesAhead)
		}
	}

	// Interpolate between current state and next state
	if g.displayStateIdx >= 0 && g.displayStateIdx < len(g.stateBuffer) {
		currentState := g.stateBuffer[g.displayStateIdx].state
		currentTurn := float64(currentState.Turn)

		// Calculate interpolation factor (0.0 to 1.0)
		elapsed := now.Sub(g.animationStart)
		t := float64(elapsed) / float64(turnPlaybackRate)
		if t > 1.0 {
			t = 1.0
		}

		// Apply easing for smooth movement
		// t = g.easeInOutQuad(t)

		// Interpolate turn number
		if g.displayStateIdx+1 < len(g.stateBuffer) {
			nextTurn := float64(g.stateBuffer[g.displayStateIdx+1].state.Turn)
			g.currentTurn = currentTurn + (nextTurn-currentTurn)*t
		} else {
			// No next state, just stay at current
			g.currentTurn = currentTurn
		}

		g.turnProgress = t
	}
}

func (g *Game) updateDisplayState() {
	var currentState *api.State

	// Get current state from appropriate source
	if g.isLiveMode && g.displayStateIdx >= 0 && g.displayStateIdx < len(g.stateBuffer) {
		currentState = g.stateBuffer[g.displayStateIdx].state
	} else if !g.isLiveMode && g.currentStateIdx < len(g.states) {
		currentState = g.states[g.currentStateIdx]
	} else if len(g.states) > 0 && g.currentStateIdx < len(g.states) {
		currentState = g.states[g.currentStateIdx]
	} else {
		return
	}

	// Update cities
	for name, city := range currentState.Cities {
		if display, exists := g.cities[name]; exists {
			display.Player = city.Player
			display.Troops = city.Troops
			display.Size = g.calculateCitySize(city.Troops)
		}
	}

	g.updateMovements()
}

func (g *Game) updateMovements() {
	var currentState *api.State

	// Get current state from appropriate source
	if g.isLiveMode && g.displayStateIdx >= 0 && g.displayStateIdx < len(g.stateBuffer) {
		currentState = g.stateBuffer[g.displayStateIdx].state
	} else if !g.isLiveMode && g.currentStateIdx < len(g.states) {
		currentState = g.states[g.currentStateIdx]
	} else if len(g.states) > 0 && g.currentStateIdx < len(g.states) {
		currentState = g.states[g.currentStateIdx]
	} else {
		return
	}

	// Update movements
	g.movements = nil

	// Use currentTurn for time calculation
	currentTime := g.currentTurn

	for _, movement := range currentState.Movements {
		fromCity, fromExists := g.cities[movement.From]
		switch to := movement.To.(type) {
		case *api.Movement_City:
			toCity, toExists := g.cities[to.City]

			if fromExists && toExists {
				// Create unique movement ID
				movementID := getMovementID(movement)

				// Get or set start turn for this movement
				startTurn, exists := g.movementStartMap[movementID]
				if !exists {
					// First time seeing this movement - record current turn as start
					startTurn = currentState.Turn
					g.movementStartMap[movementID] = startTurn
				}

				// Calculate smooth progress based on continuous time
				totalDuration := float64(movement.ArrivingTurn - startTurn)
				var progress float64
				if totalDuration > 0 {
					elapsed := currentTime - float64(startTurn)
					progress = elapsed / totalDuration
				} else {
					progress = 0.0
				}

				// Clamp progress
				if progress > 1.0 {
					progress = 1.0
				}
				if progress < 0.0 {
					progress = 0.0
				}

				// Only show movements that are in progress
				if progress >= 0.0 && progress <= 1.0 {
					g.movements = append(g.movements, &MovementDisplay{
						From:         fromCity,
						To:           toCity,
						Troops:       movement.Troops,
						Player:       movement.Player,
						StartTurn:    startTurn,
						ArrivingTurn: movement.ArrivingTurn,
						Progress:     progress,
					})
				}
			}
		case *api.Movement_Mine:
			// Currently ignoring mine movements for display
		}
	}
}

// getMovementID creates a unique identifier for a movement
func getMovementID(movement *api.Movement) string {
	var toStr string
	switch to := movement.To.(type) {
	case *api.Movement_City:
		toStr = to.City
	case *api.Movement_Mine:
		toStr = "mine_" + to.Mine
	}
	return fmt.Sprintf("%s->%s@%d:%s", movement.From, toStr, movement.ArrivingTurn, movement.Player)
}

func (g *Game) calculateCitySize(troops Troops) float64 {
	total := int64(0)
	for _, count := range troops {
		total += count
	}

	if total == 0 {
		return minCitySize
	}

	// Scale between min and max city size
	scale := float64(total) / 30.0 // Assume 30 troops = max size
	if scale > 1.0 {
		scale = 1.0
	}

	return minCitySize + (maxCitySize-minCitySize)*scale
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{20, 20, 30, 255})

	g.drawBackground(screen)

	// Calculate scale factor for game elements
	scale := float64(g.screenWidth) / float64(screenWidth)

	// Draw cities
	for _, city := range g.cities {
		g.drawCity(screen, city, scale)
	}

	// Draw movements
	for _, movement := range g.movements {
		g.drawMovement(screen, movement, scale)
	}

	// Draw turn counter and mode
	mode := "Replay"
	if g.isLiveMode {
		mode = "Live"
		if g.displayStateIdx >= 0 && g.displayStateIdx < len(g.stateBuffer) {
			turn := g.stateBuffer[g.displayStateIdx].state.Turn
			bufferInfo := fmt.Sprintf(" (buffer: %d/%d, progress: %.0f%%)",
				g.displayStateIdx+1, len(g.stateBuffer), g.turnProgress*100)
			ebitenutil.DebugPrint(screen, fmt.Sprintf("Turn: %d [%s]%s", turn, mode, bufferInfo))
		} else {
			buffered := len(g.stateBuffer)
			needed := minBufferStates
			ebitenutil.DebugPrint(screen, fmt.Sprintf("Buffering... [%s] (%d/%d states)", mode, buffered, needed))
		}
	} else if g.currentStateIdx < len(g.states) {
		turn := g.states[g.currentStateIdx].Turn
		ebitenutil.DebugPrint(screen, fmt.Sprintf("Turn: %d [%s]", turn, mode))
	} else {
		ebitenutil.DebugPrint(screen, fmt.Sprintf("Waiting for game... [%s]", mode))
	}

	// Draw player statistics panel
	g.drawPlayerStats(screen)

	// Draw game dropdown in live mode
	if g.isLiveMode {
		g.drawGameDropdown(screen)
	}
}

func (g *Game) drawGameDropdown(screen *ebiten.Image) {
	dropdownX := 10
	dropdownY := g.dropdownY
	dropdownWidth := 200
	dropdownHeight := 25

	// Draw dropdown background
	dropdownBg := ebiten.NewImage(dropdownWidth, dropdownHeight)
	dropdownBg.Fill(color.RGBA{40, 40, 50, 230})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(dropdownX), float64(dropdownY))
	screen.DrawImage(dropdownBg, op)

	// Draw border
	vector.StrokeRect(screen, float32(dropdownX), float32(dropdownY), 
		float32(dropdownWidth), float32(dropdownHeight), 1, 
		color.RGBA{100, 100, 120, 255}, false)

	// Draw selected game text
	gameText := "No game"
	if g.gameID != "" {
		gameText = g.gameID
		if len(gameText) > 25 {
			gameText = gameText[:22] + "..."
		}
	}
	ebitenutil.DebugPrintAt(screen, gameText, dropdownX+5, dropdownY+8)

	// Draw arrow indicator
	arrowX := dropdownX + dropdownWidth - 15
	arrowY := dropdownY + 12
	if g.dropdownExpanded {
		ebitenutil.DebugPrintAt(screen, "^", arrowX, arrowY-2)
	} else {
		ebitenutil.DebugPrintAt(screen, "v", arrowX, arrowY-2)
	}

	// Draw expanded options
	if g.dropdownExpanded && len(g.availableGames) > 0 {
		optionHeight := 20
		totalHeight := len(g.availableGames) * optionHeight
		
		// Draw options background
		optionsBg := ebiten.NewImage(dropdownWidth, totalHeight)
		optionsBg.Fill(color.RGBA{30, 30, 40, 240})
		optionsOp := &ebiten.DrawImageOptions{}
		optionsOp.GeoM.Translate(float64(dropdownX), float64(dropdownY+dropdownHeight))
		screen.DrawImage(optionsBg, optionsOp)

		// Draw border
		vector.StrokeRect(screen, float32(dropdownX), float32(dropdownY+dropdownHeight), 
			float32(dropdownWidth), float32(totalHeight), 1, 
			color.RGBA{100, 100, 120, 255}, false)

		// Draw each game option
		for i, gameID := range g.availableGames {
			optionY := dropdownY + dropdownHeight + i*optionHeight
			
			// Highlight selected game
			if gameID == g.gameID {
				highlight := ebiten.NewImage(dropdownWidth-2, optionHeight-1)
				highlight.Fill(color.RGBA{60, 60, 80, 255})
				highlightOp := &ebiten.DrawImageOptions{}
				highlightOp.GeoM.Translate(float64(dropdownX+1), float64(optionY+1))
				screen.DrawImage(highlight, highlightOp)
			}
			
			// Draw game ID text
			displayText := gameID
			if len(displayText) > 25 {
				displayText = displayText[:22] + "..."
			}
			ebitenutil.DebugPrintAt(screen, displayText, dropdownX+5, optionY+5)
		}
	}
}

func (g *Game) drawPlayerStats(screen *ebiten.Image) {
	// Draw statistics graph showing troops over time (Age of Empires style)
	if len(g.states) < 2 {
		return // Need at least 2 states to draw a graph
	}

	// Calculate player statistics from history
	type PlayerHistory struct {
		TotalUnits []int64
		Castles    []int
		Turns      []int64
	}

	history := make(map[string]*PlayerHistory)

	// Initialize history for all players
	for _, player := range g.playerList {
		history[player] = &PlayerHistory{
			TotalUnits: make([]int64, 0),
			Castles:    make([]int, 0),
			Turns:      make([]int64, 0),
		}
	}

	// Collect historical data from all states
	for _, state := range g.states {
		playerTroops := make(map[string]int64)
		playerCastles := make(map[string]int)

		// Count troops in cities
		for _, city := range state.Cities {
			total := city.Troops["A"] + city.Troops["B"] + city.Troops["C"]
			playerTroops[city.Player] += total
			playerCastles[city.Player]++
		}

		// Count troops in movements
		for _, movement := range state.Movements {
			total := movement.Troops["A"] + movement.Troops["B"] + movement.Troops["C"]
			playerTroops[movement.Player] += total
		}

		// Record data for each player
		for player := range history {
			history[player].TotalUnits = append(history[player].TotalUnits, playerTroops[player])
			history[player].Castles = append(history[player].Castles, playerCastles[player])
			history[player].Turns = append(history[player].Turns, state.Turn)
		}
	}

	// Draw graph panel at the bottom
	graphHeight := 200.0
	graphWidth := float64(g.screenWidth) - 20
	graphX := 10.0
	graphY := float64(g.screenHeight) - graphHeight - 10

	// Draw semi-transparent background
	panelImg := ebiten.NewImage(int(graphWidth), int(graphHeight))
	panelImg.Fill(color.RGBA{0, 0, 0, 200})

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(graphX, graphY)
	screen.DrawImage(panelImg, op)

	// Draw title
	ebitenutil.DebugPrintAt(screen, "=== TROOPS OVER TIME ===", int(graphX+10), int(graphY+5))

	// Calculate graph area
	plotX := graphX + 50
	plotY := graphY + 30
	plotWidth := graphWidth - 70
	plotHeight := graphHeight - 50

	// Find max values for scaling
	maxUnits := int64(1)
	maxTurn := int64(1)
	for _, ph := range history {
		for _, units := range ph.TotalUnits {
			if units > maxUnits {
				maxUnits = units
			}
		}
		if len(ph.Turns) > 0 && ph.Turns[len(ph.Turns)-1] > maxTurn {
			maxTurn = ph.Turns[len(ph.Turns)-1]
		}
	}

	// Draw grid lines
	gridColor := color.RGBA{60, 60, 70, 255}
	for i := 0; i <= 5; i++ {
		y := plotY + float64(i)*plotHeight/5
		vector.StrokeLine(screen, float32(plotX), float32(y), float32(plotX+plotWidth), float32(y), 1, gridColor, false)

		// Y-axis labels
		labelValue := maxUnits * int64(5-i) / 5
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", labelValue), int(graphX+5), int(y-6))
	}

	// Draw axes
	axisColor := color.RGBA{150, 150, 150, 255}
	vector.StrokeLine(screen, float32(plotX), float32(plotY), float32(plotX), float32(plotY+plotHeight), 2, axisColor, false)
	vector.StrokeLine(screen, float32(plotX), float32(plotY+plotHeight), float32(plotX+plotWidth), float32(plotY+plotHeight), 2, axisColor, false)

	// Draw lines for each player
	for _, player := range g.playerList {
		ph := history[player]
		if len(ph.TotalUnits) < 2 {
			continue
		}

		playerColor := g.playerColors[player]

		// Draw line connecting points
		for i := 0; i < len(ph.TotalUnits)-1; i++ {
			x1 := plotX + (float64(ph.Turns[i])/float64(maxTurn))*plotWidth
			y1 := plotY + plotHeight - (float64(ph.TotalUnits[i])/float64(maxUnits))*plotHeight
			x2 := plotX + (float64(ph.Turns[i+1])/float64(maxTurn))*plotWidth
			y2 := plotY + plotHeight - (float64(ph.TotalUnits[i+1])/float64(maxUnits))*plotHeight

			vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 2, playerColor, false)
		}

		// Draw current value indicator
		if len(ph.TotalUnits) > 0 {
			lastIdx := len(ph.TotalUnits) - 1
			x := plotX + (float64(ph.Turns[lastIdx])/float64(maxTurn))*plotWidth
			y := plotY + plotHeight - (float64(ph.TotalUnits[lastIdx])/float64(maxUnits))*plotHeight

			// Draw circle at endpoint
			vector.DrawFilledCircle(screen, float32(x), float32(y), 4, playerColor, false)
		}
	}

	// Draw legend
	legendX := graphX + graphWidth - 150
	legendY := graphY + 25
	for i, player := range g.playerList {
		ph := history[player]
		playerColor := g.playerColors[player]

		// Draw color box
		colorBox := ebiten.NewImage(10, 10)
		colorBox.Fill(playerColor)
		colorOp := &ebiten.DrawImageOptions{}
		colorOp.GeoM.Translate(legendX, legendY+float64(i*15))
		screen.DrawImage(colorBox, colorOp)

		// Draw player name and current stats
		var currentUnits int64
		var currentCastles int
		if len(ph.TotalUnits) > 0 {
			currentUnits = ph.TotalUnits[len(ph.TotalUnits)-1]
			currentCastles = ph.Castles[len(ph.Castles)-1]
		}
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s: %d (%d)", player, currentUnits, currentCastles),
			int(legendX+15), int(legendY+float64(i*15)))
	}

	// Draw X-axis label
	ebitenutil.DebugPrintAt(screen, "Turn", int(plotX+plotWidth/2-10), int(plotY+plotHeight+15))
	ebitenutil.DebugPrintAt(screen, "Units", int(graphX+5), int(plotY-15))
}

func (g *Game) drawTroopsAtCity(screen *ebiten.Image, city *CityDisplay, scale float64) {
	troopTypes := []string{"A", "B", "C"}
	playerColor := g.playerColors[city.Player]

	for i, troopType := range troopTypes {
		count := city.Troops[troopType]
		if count > 0 {
			angle := 2 * math.Pi * float64(i) / 3 // Distribute around city
			offset := (city.Size/2 + 3) * scale    // Even closer to castle (was 8)
			troopX := city.X*scale + offset*math.Cos(angle)
			troopY := city.Y*scale + offset*math.Sin(angle)

			// Draw troop sprite with player color tint
			troopSprite := NewTroopSprite(troopType)
			r := float64(playerColor.R) / 255.0
			gVal := float64(playerColor.G) / 255.0
			b := float64(playerColor.B) / 255.0
			troopSprite.Draw(screen, g.tickCounter, troopX, troopY, scale, r, gVal, b, 1.0)
		}
	}
}

func (g *Game) drawMovement(screen *ebiten.Image, movement *MovementDisplay, scale float64) {
	// Calculate current position
	startX, startY := movement.From.X*scale, movement.From.Y*scale
	endX, endY := movement.To.X*scale, movement.To.Y*scale

	currentX := startX + (endX-startX)*movement.Progress
	currentY := startY + (endY-startY)*movement.Progress

	playerColor := g.playerColors[movement.Player]
	troopTypes := []string{"A", "B", "C"}

	// Draw troops in formation pointing toward destination
	for i, troopType := range troopTypes {
		count := movement.Troops[troopType]
		if count > 0 {
			// Offset troops slightly to avoid overlap
			offsetX := currentX + float64((i-1)*8)*scale
			offsetY := currentY + float64((i-1)*8)*scale

			// Draw troop sprite with player color tint
			troopSprite := NewTroopSprite(troopType)
			r := float64(playerColor.R) / 255.0
			gVal := float64(playerColor.G) / 255.0
			b := float64(playerColor.B) / 255.0
			troopSprite.Draw(screen, g.tickCounter, offsetX, offsetY, scale, r, gVal, b, 1.0)
		}
	}
}

// drawFilledOval draws a filled oval/ellipse at the given center with the specified radii
func drawFilledOval(screen *ebiten.Image, centerX, centerY, radiusX, radiusY float32, col color.RGBA) {
	// Draw an approximation using a path with many points
	var path vector.Path
	segments := 32
	for i := 0; i <= segments; i++ {
		angle := 2 * math.Pi * float64(i) / float64(segments)
		x := centerX + radiusX*float32(math.Cos(angle))
		y := centerY + radiusY*float32(math.Sin(angle))
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	path.Close()

	vertices, indices := path.AppendVerticesAndIndicesForFilling(nil, nil)
	for i := range vertices {
		vertices[i].ColorR = float32(col.R) / 255.0
		vertices[i].ColorG = float32(col.G) / 255.0
		vertices[i].ColorB = float32(col.B) / 255.0
		vertices[i].ColorA = float32(col.A) / 255.0
	}

	// Initialize emptySubImage if needed
	if emptySubImage == nil {
		emptySubImage = ebiten.NewImage(3, 3)
		emptySubImage.Fill(color.White)
	}

	screen.DrawTriangles(vertices, indices, emptySubImage, &ebiten.DrawTrianglesOptions{
		FillRule: ebiten.FillRuleNonZero,
	})
}

var emptySubImage *ebiten.Image

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	// Make it square based on the smaller dimension
	size := min(outsideHeight, outsideWidth)
	// Update the game's screen dimensions
	g.screenWidth = size
	g.screenHeight = size
	return size, size
}

func main() {
	var game *Game
	var err error

	// Usage:
	//   ui-engine-v2                              # watch mode
	//   ui-engine-v2 <file>                       # replay mode

	if len(os.Args) < 2 {
		log.Println("No file specified, connecting to live server...")
		apiURL := defaultAPIURL
		if len(os.Args) == 2 {
			apiURL = os.Args[1]
		}
		game, err = NewGameLive(apiURL)
		if err != nil {
			log.Fatalf("Failed to connect to server: %v", err)
		}
		log.Printf("Connected to server at %s", apiURL)
	} else {
		log.Printf("Loading replay from %s", os.Args[1])
		game, err = NewGame(os.Args[1])
		if err != nil {
			log.Fatalf("Failed to load replay: %v", err)
		}
	}

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Mask Invaders Visualization")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(60)
	ebiten.SetVsyncEnabled(true)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
