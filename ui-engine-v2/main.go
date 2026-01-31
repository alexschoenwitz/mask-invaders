package main

import (
	"bytes"
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
	minTroopSize     = 8
	maxTroopSize     = 10
	pollInterval     = 200 * time.Millisecond // How often to poll the server
	turnPlaybackRate = 500 * time.Millisecond // Fixed rate to consume states from buffer
	minBufferStates  = 3                      // Minimum states to buffer before starting playback
	stateBufferSize  = 20                     // Number of states to keep in buffer
	defaultAPIURL    = "http://localhost:8080"
)

// Game state structures - using API protobuf types
type Troops map[string]int64

// Display structures
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
	humanUI          *HumanUI         // Human interaction UI (optional)
}

// Player colors palette
var colors = []color.RGBA{
	{255, 0, 0, 255},     // Red
	{0, 255, 0, 255},     // Green
	{0, 0, 255, 255},     // Blue
	{255, 255, 0, 255},   // Yellow
	{255, 0, 255, 255},   // Magenta
	{0, 255, 255, 255},   // Cyan
	{255, 128, 0, 255},   // Orange
	{128, 0, 255, 255},   // Purple
	{255, 192, 203, 255}, // Pink
	{0, 128, 0, 255},     // Dark Green
	{128, 128, 128, 255}, // Gray
	{255, 255, 255, 255}, // White
	{128, 0, 0, 255},     // Maroon
	{0, 128, 128, 255},   // Teal
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
	}

	// Fetch historical data first for the graph
	if err := game.fetchStateHistory(); err != nil {
		log.Printf("Warning: Failed to fetch state history: %v", err)
	}

	// Try to fetch initial state
	if err := game.pollServerState(); err != nil {
		log.Printf("Warning: Failed to fetch initial state: %v", err)
	}

	return game, nil
}

func (g *Game) fetchStateHistory() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", g.apiURL+"/v1/state:history", nil)
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

func (g *Game) pollServerState() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", g.apiURL+"/v1/state", nil)
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

	// Update human UI if present
	if g.humanUI != nil {
		if err := g.humanUI.Update(); err != nil {
			return err
		}
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

func (g *Game) calculateTroopSize(count int64) float64 {
	if count == 0 {
		return 0
	}

	scale := float64(count) / 10.0 // Assume 10 troops = max size
	if scale > 1.0 {
		scale = 1.0
	}
	if scale < 0.1 {
		scale = 0.1 // Minimum visibility
	}

	return minTroopSize + (maxTroopSize-minTroopSize)*scale
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

	// Draw human UI if present
	if g.humanUI != nil {
		g.humanUI.Draw(screen)
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
			offset := (city.Size/2 + 15) * scale
			troopX := city.X*scale + offset*math.Cos(angle)
			troopY := city.Y*scale + offset*math.Sin(angle)

			// Draw oval shadow marker below troop
			// Make color less strong (30% opacity)
			shadowColor := color.RGBA{
				R: playerColor.R,
				G: playerColor.G,
				B: playerColor.B,
				A: 76, // 30% of 255
			}
			// Center better (add sprite width offset), smaller size, more down
			ovalCenterX := float32(troopX + 8*scale)  // Center it better
			ovalCenterY := float32(troopY + 20*scale) // Move it more down
			ovalRadiusX := float32(12 * scale)        // Smaller width
			ovalRadiusY := float32(5 * scale)         // Smaller height
			drawFilledOval(screen, ovalCenterX, ovalCenterY, ovalRadiusX, ovalRadiusY, shadowColor)

			// Draw troop sprite without tint
			troopSprite := NewTroopSprite(troopType)
			troopSprite.Draw(screen, g.tickCounter, troopX, troopY, scale, 1.0, 1.0, 1.0, 1.0)
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

			// Draw oval shadow marker below troop
			// Make color less strong (30% opacity)
			shadowColor := color.RGBA{
				R: playerColor.R,
				G: playerColor.G,
				B: playerColor.B,
				A: 76, // 30% of 255
			}
			// Center better (add sprite width offset), smaller size, more down
			ovalCenterX := float32(offsetX + 8*scale)  // Center it better
			ovalCenterY := float32(offsetY + 20*scale) // Move it more down
			ovalRadiusX := float32(12 * scale)         // Smaller width
			ovalRadiusY := float32(5 * scale)          // Smaller height
			drawFilledOval(screen, ovalCenterX, ovalCenterY, ovalRadiusX, ovalRadiusY, shadowColor)

			// Draw troop sprite without tint
			troopSprite := NewTroopSprite(troopType)
			troopSprite.Draw(screen, g.tickCounter, offsetX, offsetY, scale, 1.0, 1.0, 1.0, 1.0)
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
		FillRule: ebiten.NonZero,
	})
}

var emptySubImage *ebiten.Image

func (g *Game) drawTroop(screen *ebiten.Image, troopType string, x, y, size float32, playerColor color.RGBA) {
	// No longer used - kept for compatibility
}

func (g *Game) drawOrientedTroop(screen *ebiten.Image, troopType string, x, y, size, angle float32, playerColor color.RGBA) {
	// No longer used - kept for compatibility
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	// Make it square based on the smaller dimension
	size := outsideWidth
	if outsideHeight < outsideWidth {
		size = outsideHeight
	}
	// Update the game's screen dimensions
	g.screenWidth = size
	g.screenHeight = size
	return size, size
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func newJSONRequest(ctx context.Context, method, url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func main() {
	var game *Game
	var err error

	// Check command line arguments for human mode
	// Usage:
	//   ui-engine-v2                              # watch mode
	//   ui-engine-v2 <file>                       # replay mode
	//   ui-engine-v2 --human <player-name>        # human interactive mode
	var playerName string
	var playerToken string

	if len(os.Args) >= 3 && os.Args[1] == "--human" {
		playerName = os.Args[2]

		// Register the player
		apiURL := defaultAPIURL
		if len(os.Args) >= 4 {
			apiURL = os.Args[3]
		}

		game, err = NewGameLive(apiURL)
		if err != nil {
			log.Fatalf("Failed to connect to server: %v", err)
		}

		// Register player
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		registerReq := &api.RegisterRequest{Name: playerName}
		jsonData, err := protojson.Marshal(registerReq)
		if err != nil {
			log.Fatalf("Failed to marshal register request: %v", err)
		}

		req, err := newJSONRequest(ctx, "POST", apiURL+"/v1/register", jsonData)
		if err != nil {
			log.Fatalf("Failed to create register request: %v", err)
		}

		resp, err := game.httpClient.Do(req)
		if err != nil {
			log.Fatalf("Failed to register player: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			log.Fatalf("Failed to register player: status %d: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Failed to read register response: %v", err)
		}

		registerResp := &api.RegisterResponse{}
		if err := protojson.Unmarshal(body, registerResp); err != nil {
			log.Fatalf("Failed to parse register response: %v", err)
		}

		playerToken = registerResp.Token
		log.Printf("Registered as player '%s' with ID %s and token %s", playerName, registerResp.Id, playerToken)

		// Create human UI
		game.humanUI = NewHumanUI(game, playerName, registerResp.Id, playerToken)

		log.Printf("Human interactive mode enabled for player '%s'", playerName)

	} else if len(os.Args) < 2 {
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
	ebiten.SetFPSMode(ebiten.FPSModeVsyncOn)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
