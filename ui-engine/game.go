package main

import (
	"fmt"
	"image/color"
	_ "image/png"
	"math"
	"math/rand"
	"strconv"
	"time"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"github.com/hajimehoshi/ebiten/v2"
	// Needed for loading files
)

type Game struct {
	// game config
	screenWidth     int
	screenHeight    int
	tickCounter     int
	ticksPerTurn    int
	showTroopNumber bool

	// state management - some basic design philosophy
	// 1. the game is responsible for knowing about all
	// 		objects that exist in it
	// 2. the game IS NOT responsible for updating all the objects,
	//		it is only responsible for passing the up to date full state
	// 3. each object is responsible for UPDATING and DRAWING itself
	conn        *GameStateManager
	gameMap     *Map                   // UPDATED every turn
	spawnPoints map[string]*SpawnPoint // UPDATED every turn
	armies      map[string]*Army       // RE-CREATED every turn

	playerList   []string
	playerColors map[string]color.RGBA
	colorPalette []color.RGBA
}

func NewGame(d MapDistribution,
	screenWidth, screenHeight int,
	ticksPerTurn int,
) (*Game, error) {
	fmt.Println("asdasdasd------------------->", ticksPerTurn)

	conn, err := NewGameStateManager("")
	if err != nil {
		return nil, err
	}
	// Game will only be created if there are the min number of turns
	for !conn.isReadyToRenderGame() {
		err = conn.pollServerAndUpdateBuffer()
		if err != nil {
			return nil, err
		}
	}

	err = conn.setToFirstTurn()
	if err != nil {
		return nil, err
	}

	sps, err := createSpawnPoints(d, conn.currentState, screenWidth, screenHeight)
	if err != nil {
		return nil, err
	}

	game := &Game{
		screenWidth:  screenWidth,
		screenHeight: screenHeight,
		tickCounter:  0,
		ticksPerTurn: ticksPerTurn,

		conn:            conn,
		gameMap:         NewMap(screenWidth, screenHeight),
		armies:          map[string]*Army{},
		spawnPoints:     sps,
		showTroopNumber: true,
	}

	return game, nil
}

func (g *Game) Update() error {
	var isNewTurn = false
	fmt.Printf("tick is: %d, turn is: %d\n", g.tickCounter, g.conn.currentTurn)
	g.tickCounter++
	g.conn.pollServerAndUpdateBuffer()
	if g.tickCounter%g.ticksPerTurn == 0 {
		isNewTurn = g.conn.hasNextTurn()
	}
	// we only need to update state if
	// we have a new turn
	if !isNewTurn {
		return nil
	}

	// TODO(PC): This nil check can be removed
	// once we haved logic to handle "game over"
	if g.conn.currentState == nil {
		return nil
	}

	// update spawn points
	// TODO(PC): in the future it might possible to create new spawn points
	for id, sp := range g.spawnPoints {
		if _, ok := g.spawnPoints[id]; ok {
			err := sp.Update(g.conn.currentState)
			if err != nil {
				return err
			}
		}
	}

	g.updateArmies()

	return nil
}

func (g *Game) updateArmies() error {
	// TODO(PC): figure out color based on player
	ownerColor := color.RGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF}

	// first we clean up any armies that have arrived their destination
	for id, a := range g.armies {
		if a.deleteResource {
			delete(g.armies, id)
		}
	}

	// then we create new armies based on the current turn movements
	for _, m := range g.conn.currentState.GetMovements() {
		// TODO(PC): ID should come from the backend
		armyId := m.GetFrom() + m.GetCity() + strconv.Itoa(int(m.GetArrivingTurn()))

		if _, ok := g.armies[armyId]; !ok {
			selectedDistance := findDistance(m.GetFrom(), m.GetCity(), g.conn.currentState.GetDistances())
			sFrom := g.spawnPoints[m.GetFrom()]
			sTo := g.spawnPoints[m.GetCity()]

			a := NewArmy(m, selectedDistance, sFrom.P, sTo.P, ownerColor, g.showTroopNumber)
			g.armies[armyId] = a
		}
	}

	fmt.Println("There are: ", len(g.armies), " armies!")
	for _, army := range g.armies {

		army.Debug()
	}

	return nil
}

// TODO(PC): remove after improving BE reponse
// this part is absolute garbage because the API is not so nice to transverse
// after merging to main we need to clean up the API and improve this
//
// a -> b can be defined in the distance map as ab or ba .... funny one Luis
func findDistance(from, to string, distances map[string]*api.Distance) *api.Distance {
	dKey1 := from + to
	dKey2 := to + from
	var selectedDistance *api.Distance
	if d, ok := distances[dKey1]; ok {
		selectedDistance = d
	}
	if d, ok := distances[dKey2]; ok {
		selectedDistance = d
	}
	return selectedDistance
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.gameMap.Draw(screen, g.tickCounter)

	if !g.conn.isReadyToRenderGame() {
		return
	}

	for _, o := range g.spawnPoints {
		o.Draw(screen, g.tickCounter)
	}

	for _, o := range g.armies {
		if o != nil {
			o.Draw(screen, g.tickCounter)

		}
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return g.screenWidth, g.screenHeight
}

type Point struct {
	x float64
	y float64
}

type MapDistribution int

const (
	RectangularDistribution MapDistribution = iota
	CircularDistribution
	FancyDistribution
)

func createSpawnPoints(
	d MapDistribution, st *api.State,
	screenWidth, screenHeight int,
) (map[string]*SpawnPoint, error) {
	totalSpawnPoints := len(st.GetCities())
	ps := make([]Point, totalSpawnPoints)
	out := make(map[string]*SpawnPoint, len(st.GetCities()))

	switch d {
	case RectangularDistribution:
		ps = rectangularDistribution(totalSpawnPoints, float64(screenWidth), float64(screenHeight))
	case CircularDistribution:
		ps = circularDistribution(totalSpawnPoints, float64(screenWidth), float64(screenHeight))
	case FancyDistribution:
		ds := []*api.Distance{}
		for _, d := range st.GetDistances() {
			ds = append(ds, d)
		}
		cs, err := generateMapCoordinates(ds, float64(screenWidth), float64(screenHeight))
		if err != nil {
			return nil, err
		}

		for id, pt := range cs {
			sp, err := NewSpawnPoint(pt, id, st)
			if err != nil {
				return nil, err
			}
			out[id] = sp
		}
		return out, nil
	}

	psX := 0
	for id := range st.GetCities() {
		sp, err := NewSpawnPoint(ps[psX], id, st)
		if err != nil {
			return nil, err
		}
		out[id] = sp
		psX++
	}

	return out, nil
}

func rectangularDistribution(n int, mapWidth, mapHeight float64) []Point {
	points := make([]Point, n)

	// 1. Calculate the best grid size (Columns x Rows)
	// For 6 points on a wide map, 3 cols x 2 rows is best.
	// We use Sqrt to find a balanced grid.
	cols := int(math.Ceil(math.Sqrt(float64(n))))
	rows := int(math.Ceil(float64(n) / float64(cols)))

	// Calculate the size of each "cell" in the grid
	cellWidth := mapWidth / float64(cols)
	cellHeight := mapHeight / float64(rows)

	for i := 0; i < n; i++ {
		// Calculate which Row and Column this point belongs to
		row := i / cols // Integer division (0, 0, 0, 1, 1, 1)
		col := i % cols // Remainder (0, 1, 2, 0, 1, 2)

		// Calculate X and Y to be in the CENTER of that cell
		x := (float64(col) * cellWidth) + (cellWidth / 2)
		y := (float64(row) * cellHeight) + (cellHeight / 2)

		points[i] = Point{x: x, y: y}
	}

	return points
}

func circularDistribution(n int, width, height float64) []Point {
	points := make([]Point, n)

	// 1. Find the center of the map
	centerX := width / 2
	centerY := height / 2

	// 2. Determine Radius (keep it inside the map bounds)
	// We use the smaller dimension divided by 3 to leave some padding
	radius := math.Min(width, height) / 3

	// 3. Calculate the angle step (in radians)
	// Full circle is 2*Pi. Divide by N.
	angleStep := (2 * math.Pi) / float64(n)

	for i := 0; i < n; i++ {
		// Calculate the angle for this specific point
		// We subtract Pi/2 to start at the top (12 o'clock position)
		currentAngle := (angleStep * float64(i)) - (math.Pi / 2)

		// 4. Polar to Cartesian coordinates
		// X = cx + r * cos(a)
		// Y = cy + r * sin(a)
		x := centerX + (radius * math.Cos(currentAngle))
		y := centerY + (radius * math.Sin(currentAngle))

		points[i] = Point{x: x, y: y}
	}

	return points
}

// Node represents a graph node for calculation
type Node struct {
	ID string
	X  float64
	Y  float64
}

// Edge represents a connection with a target distance
type Edge struct {
	SourceID   string
	TargetID   string
	TargetDist float64
}

// GenerateMapCoordinates is the main function you requested
func generateMapCoordinates(c []*api.Distance, width float64, height float64) (map[string]Point, error) {
	// 1. Parse Nodes and Edges
	nodes := make(map[string]*Node)
	var edges []Edge

	// Based on your data: "City-" (5) + UUID (36) + "-" (1) + Index (1) = 43 chars
	const IDLength = 43

	for _, val := range c {
		id1 := val.Edge[:IDLength]
		id2 := val.Edge[IDLength:]

		dist := float64(val.GetDistance())

		if _, exists := nodes[id1]; !exists {
			nodes[id1] = &Node{ID: id1}
		}
		if _, exists := nodes[id2]; !exists {
			nodes[id2] = &Node{ID: id2}
		}

		edges = append(edges, Edge{SourceID: id1, TargetID: id2, TargetDist: dist})
	}

	// 2. Initialize Random Positions
	rand.Seed(time.Now().UnixNano())
	nodeList := []*Node{}
	for _, n := range nodes {
		n.X = rand.Float64() * 10.0
		n.Y = rand.Float64() * 10.0
		nodeList = append(nodeList, n)
	}

	// 3. Run Force-Directed Simulation (Spring Embedding)
	// We iteratively move nodes to satisfy the 'TargetDist'
	for i := 0; i < 5000; i++ {
		// Reduce movement speed over time (Annealing)
		alpha := 0.1 * (1.0 - float64(i)/float64(5000))

		for _, edge := range edges {
			n1 := nodes[edge.SourceID]
			n2 := nodes[edge.TargetID]

			dx := n2.X - n1.X
			dy := n2.Y - n1.Y
			currentDist := math.Sqrt(dx*dx + dy*dy)

			if currentDist == 0 {
				currentDist = 0.0001 // Prevent division by zero
				dx = 0.0001
			}

			// Spring Force: pull or push based on difference from target
			// displacement = difference between current distance and desired distance
			displacement := currentDist - edge.TargetDist

			// Calculate movement vector
			moveX := (dx / currentDist) * displacement * alpha
			moveY := (dy / currentDist) * displacement * alpha

			// Move nodes towards equilibrium
			n1.X += moveX
			n1.Y += moveY
			n2.X -= moveX
			n2.Y -= moveY
		}
	}

	// 4. Normalize to 1000x800 Canvas
	minX, maxX := math.MaxFloat64, -math.MaxFloat64
	minY, maxY := math.MaxFloat64, -math.MaxFloat64

	for _, n := range nodeList {
		if n.X < minX {
			minX = n.X
		}
		if n.X > maxX {
			maxX = n.X
		}
		if n.Y < minY {
			minY = n.Y
		}
		if n.Y > maxY {
			maxY = n.Y
		}
	}

	currW := maxX - minX
	currH := maxY - minY

	// Avoid division by zero if all points are stacked
	if currW == 0 {
		currW = 1
	}
	if currH == 0 {
		currH = 1
	}

	// Calculate Scale to fit in box with padding
	availW := width - (2 * 75)
	availH := height - (2 * 75)

	scaleX := availW / currW
	scaleY := availH / currH

	// Use the smaller scale to maintain aspect ratio
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	// Calculate centering offsets
	finalW := currW * scale
	finalH := currH * scale
	offsetX := (width - finalW) / 2
	offsetY := (height - finalH) / 2

	// 5. Build Final Output
	output := make(map[string]Point)

	// Sort keys for deterministic iteration order if printing (optional)
	// Go maps are random iteration, but the math is already done.

	for _, n := range nodeList {
		// Normalize 0..1 relative to bounds
		relX := n.X - minX
		relY := n.Y - minY

		// Scale and Pad
		screenX := int(offsetX + (relX * scale))
		screenY := int(offsetY + (relY * scale))

		output[n.ID] = Point{x: float64(screenX), y: float64(screenY)}
	}

	return output, nil
}
