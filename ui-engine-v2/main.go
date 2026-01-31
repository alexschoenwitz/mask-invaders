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
	isLiveMode       bool          // true if using live API, false if replay mode
	animationStart   time.Time     // When we started animating current turn
	turnDuration     time.Duration // Duration for current turn animation
	screenWidth      int           // Current screen width
	screenHeight     int           // Current screen height
	movementStartMap map[string]int64 // Cache of movement ID -> start turn
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

	// Try to fetch initial state
	if err := game.pollServerState(); err != nil {
		log.Printf("Warning: Failed to fetch initial state: %v", err)
	}

	return game, nil
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

		// Also add to states list for compatibility
		g.states = append(g.states, stateResponse.State)
		if len(g.states) > stateBufferSize {
			g.states = g.states[1:]
		}

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
	now := time.Now()

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
}

func (g *Game) drawTroopsAtCity(screen *ebiten.Image, city *CityDisplay, scale float64) {
	troopTypes := []string{"A", "B", "C"}
	playerColor := g.playerColors[city.Player]
	tintR := float64(playerColor.R) / 255.0
	tintG := float64(playerColor.G) / 255.0
	tintB := float64(playerColor.B) / 255.0

	for i, troopType := range troopTypes {
		count := city.Troops[troopType]
		if count > 0 {
			angle := 2 * math.Pi * float64(i) / 3 // Distribute around city
			offset := (city.Size/2 + 15) * scale
			troopX := city.X*scale + offset*math.Cos(angle)
			troopY := city.Y*scale + offset*math.Sin(angle)

			troopSprite := NewTroopSprite(troopType)
			troopSprite.Draw(screen, int(g.currentTurn), troopX, troopY, scale, tintR, tintG, tintB, 1.0)
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
	tintR := float64(playerColor.R) / 255.0
	tintG := float64(playerColor.G) / 255.0
	tintB := float64(playerColor.B) / 255.0
	troopTypes := []string{"A", "B", "C"}

	// Draw troops in formation pointing toward destination
	for i, troopType := range troopTypes {
		count := movement.Troops[troopType]
		if count > 0 {
			// Offset troops slightly to avoid overlap
			offsetX := currentX + float64((i-1)*8)*scale
			offsetY := currentY + float64((i-1)*8)*scale

			troopSprite := NewTroopSprite(troopType)
			troopSprite.Draw(screen, int(g.currentTurn), offsetX, offsetY, scale, tintR, tintG, tintB, 1.0)
		}
	}
}

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

func main() {
	var game *Game
	var err error

	// Check if running in live mode or replay mode
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
	ebiten.SetFPSMode(ebiten.FPSModeVsyncOn)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
