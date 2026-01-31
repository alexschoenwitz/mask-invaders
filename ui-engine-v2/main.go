package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"math"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
)

const (
	screenWidth  = 1200
	screenHeight = 800
	minCitySize  = 50
	maxCitySize  = 60
	minTroopSize = 8
	maxTroopSize = 10
	turnDuration = 100 * time.Millisecond
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

// Game represents the main game state
type Game struct {
	states          []*api.State
	currentStateIdx int
	lastUpdate      time.Time
	turnProgress    float64 // 0.0 to 1.0 progress within current turn
	currentTurn     float64 // Continuous current turn with sub-turn precision
	cities          map[string]*CityDisplay
	movements       []*MovementDisplay
	playerColors    map[string]color.RGBA
	colorPalette    []color.RGBA
	playerList      []string
	frameCount      int
	offscreen       *ebiten.Image
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
		states:       states,
		lastUpdate:   time.Now(),
		cities:       make(map[string]*CityDisplay),
		playerColors: make(map[string]color.RGBA),
		colorPalette: colors,
		offscreen:    ebiten.NewImage(screenWidth, screenHeight),
	}

	game.initializeCities()
	game.assignPlayerColors()
	game.updateDisplayState() // Initialize the display state

	return game, nil
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

	// Arrange cities in a circle or grid
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
	elapsed := now.Sub(g.lastUpdate)

	// Update turn progress for smooth transitions
	g.turnProgress = float64(elapsed) / float64(turnDuration)

	// Update continuous current turn for smooth movement
	if len(g.states) > 0 && g.currentStateIdx < len(g.states) {
		baseTurn := float64(g.states[g.currentStateIdx].Turn)
		g.currentTurn = baseTurn + g.turnProgress
	}

	if elapsed >= turnDuration {
		g.lastUpdate = now
		g.turnProgress = 0.0
		g.currentStateIdx++
		if g.currentStateIdx >= len(g.states) {
			g.currentStateIdx = len(g.states)
		}
		g.updateDisplayState()
	} else {
		g.updateMovements()
	}
	return nil
}

func (g *Game) updateDisplayState() {
	if g.currentStateIdx >= len(g.states) {
		return
	}

	currentState := g.states[g.currentStateIdx]

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
	if g.currentStateIdx >= len(g.states) {
		return
	}

	currentState := g.states[g.currentStateIdx]

	// Update movements
	g.movements = nil

	for _, movement := range currentState.Movements {
		if movement.ArrivingTurn > int64(g.currentTurn) {
			fromCity, fromExists := g.cities[movement.From]
			toCity, toExists := g.cities[movement.To]

			if fromExists && toExists {
				// Calculate movement start time (when it was created)
				startTurn := g.findMovementStartTurn(movement)

				// Calculate smooth progress based on continuous time
				totalDuration := float64(movement.ArrivingTurn - startTurn)
				elapsed := g.currentTurn - float64(startTurn)
				progress := elapsed / totalDuration

				if progress > 1.0 {
					progress = 1.0
				}
				if progress < 0.0 {
					progress = 0.0
				}

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
	}
}

// findMovementStartTurn determines when a movement first appeared in the game states
func (g *Game) findMovementStartTurn(targetMovement *api.Movement) int64 {
	// Find when this movement first appeared by looking through previous states
	for _, state := range g.states {
		for _, movement := range state.Movements {
			if movement.From == targetMovement.From &&
				movement.To == targetMovement.To &&
				movement.ArrivingTurn == targetMovement.ArrivingTurn &&
				movement.Player == targetMovement.Player {
				return state.Turn
			}
		}
	}
	// Fallback: assume it started one turn before arriving
	return targetMovement.ArrivingTurn - 1
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

	// Draw cities
	for _, city := range g.cities {
		g.drawCity(screen, city)
	}

	// Draw movements
	for _, movement := range g.movements {
		g.drawMovement(screen, movement)
	}

	// Draw turn counter
	if g.currentStateIdx < len(g.states) {
		turn := g.states[g.currentStateIdx].Turn
		ebitenutil.DebugPrint(screen, fmt.Sprintf("Turn: %d", turn))
	}
}

func (g *Game) drawCity(screen *ebiten.Image, city *CityDisplay) {
	playerColor := g.playerColors[city.Player]
	x, y := int(city.X), int(city.Y)
	size := int(city.Size)
	halfSize := size / 2

	// Draw simple city box using ebitenutil
	ebitenutil.DrawRect(screen, float64(x-halfSize), float64(y-halfSize), float64(size), 2, playerColor)
	ebitenutil.DrawRect(screen, float64(x-halfSize), float64(y+halfSize-2), float64(size), 2, playerColor)
	ebitenutil.DrawRect(screen, float64(x-halfSize), float64(y-halfSize), 2, float64(size), playerColor)
	ebitenutil.DrawRect(screen, float64(x+halfSize-2), float64(y-halfSize), 2, float64(size), playerColor)

	// Draw troops around the city
	g.drawTroopsAtCity(screen, city)
}

func (g *Game) drawTroopsAtCity(screen *ebiten.Image, city *CityDisplay) {
	troopTypes := []string{"A", "B", "C"}
	playerColor := g.playerColors[city.Player]

	for i, troopType := range troopTypes {
		count := city.Troops[troopType]
		if count > 0 {
			angle := 2 * math.Pi * float64(i) / 3 // Distribute around city
			offset := city.Size/2 + 15
			troopX := city.X + offset*math.Cos(angle)
			troopY := city.Y + offset*math.Sin(angle)
			troopSize := g.calculateTroopSize(count)

			g.drawTroop(screen, troopType, float32(troopX), float32(troopY), float32(troopSize), playerColor)
		}
	}
}

func (g *Game) drawMovement(screen *ebiten.Image, movement *MovementDisplay) {
	// Calculate current position
	startX, startY := movement.From.X, movement.From.Y
	endX, endY := movement.To.X, movement.To.Y

	currentX := startX + (endX-startX)*movement.Progress
	currentY := startY + (endY-startY)*movement.Progress

	// Calculate direction angle for troop orientation
	dirX := endX - startX
	dirY := endY - startY
	angle := math.Atan2(dirY, dirX)

	playerColor := g.playerColors[movement.Player]
	troopTypes := []string{"A", "B", "C"}

	// Draw troops in formation pointing toward destination
	for i, troopType := range troopTypes {
		count := movement.Troops[troopType]
		if count > 0 {
			// Offset troops slightly to avoid overlap
			offsetX := currentX + float64((i-1)*8)
			offsetY := currentY + float64((i-1)*8)
			troopSize := g.calculateTroopSize(count)

			g.drawOrientedTroop(screen, troopType, float32(offsetX), float32(offsetY), float32(troopSize), float32(angle), playerColor)
		}
	}
}

func (g *Game) drawTroop(screen *ebiten.Image, troopType string, x, y, size float32, playerColor color.RGBA) {
	g.drawOrientedTroop(screen, troopType, x, y, size, 0, playerColor)
}

func (g *Game) drawOrientedTroop(screen *ebiten.Image, troopType string, x, y, size, angle float32, playerColor color.RGBA) {
	// Draw simple filled rectangles for all troop types
	halfSize := float64(size / 2)
	ebitenutil.DrawRect(screen, float64(x)-halfSize, float64(y)-halfSize, float64(size), float64(size), playerColor)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run . <gamehistory.json>")
	}

	game, err := NewGame(os.Args[1])
	if err != nil {
		log.Fatalf("Failed to create game: %v", err)
	}

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Mask Invaders Replay")
	ebiten.SetTPS(60)
	ebiten.SetFPSMode(ebiten.FPSModeVsyncOn)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
