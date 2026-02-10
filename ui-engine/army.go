package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"github.com/alexschoenwitz/mask-invaders/ui-engine/resources"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

var (
	gopherImage   *ebiten.Image
	archerImage   *ebiten.Image
	knightImage   *ebiten.Image
	infantryImage *ebiten.Image
)

// loads images before starting
// always re-use image, don't load new ones
func init() {
	img, _, err := image.Decode(bytes.NewReader(resources.Gopher_png))
	if err != nil {
		log.Fatal(err)
	}
	gopherImage = ebiten.NewImageFromImage(img)

	img, _, err = image.Decode(bytes.NewReader(resources.Archer_png))
	if err != nil {
		log.Fatal(err)
	}
	archerImage = ebiten.NewImageFromImage(img)

	img, _, err = image.Decode(bytes.NewReader(resources.Knight_png))
	if err != nil {
		log.Fatal(err)
	}
	knightImage = ebiten.NewImageFromImage(img)

	img, _, err = image.Decode(bytes.NewReader(resources.Infantry_png))
	if err != nil {
		log.Fatal(err)
	}
	infantryImage = ebiten.NewImageFromImage(img)

	img, _, err = image.Decode(bytes.NewReader(resources.Castle_png))
	if err != nil {
		log.Fatal(err)
	}
	castleImage = ebiten.NewImageFromImage(img)
}

type troopType int

const (
	Gopher troopType = iota
	Archer
	Knight
	Infantry
	CastleTmp
)

type Army struct {
	// thing to move and update state
	x, y                                           float64
	startX, startY                                 float64
	targetX, targetY                               float64
	turnOfCreation, turnOfArrival, distanceInTurns int

	deleteResource bool // tells the engine if the object can be de-referenced

	// things to draw
	knights       *Troop
	knightsCount  int
	archers       *Troop
	archersCount  int
	infantry      *Troop
	infantryCount int

	// things to debug
	showTroopCounter bool
	ownerColor       color.RGBA
}

func NewArmy(
	m *api.Movement,
	distance *api.Distance,
	coordFrom, coordTo Point,
	ownerColor color.RGBA,
	showTroopCounter bool,
) *Army {
	var archersCount, infantryCount, knightsCount int
	for tt, nr := range m.GetTroops() {
		switch tt {
		case "A":
			archersCount = int(nr)
		case "B":
			infantryCount = int(nr)
		case "C":
			knightsCount = int(nr)
		}
	}

	startingTurn := m.GetArrivingTurn() - distance.GetDistance()

	a := &Army{
		knights:       NewTroop(Knight),
		knightsCount:  knightsCount,
		archers:       NewTroop(Archer),
		archersCount:  archersCount,
		infantry:      NewTroop(Infantry),
		infantryCount: infantryCount,

		startX:          coordFrom.x,
		startY:          coordFrom.y,
		targetX:         coordTo.x,
		targetY:         coordTo.y,
		turnOfCreation:  int(startingTurn),
		turnOfArrival:   int(m.GetArrivingTurn()),
		distanceInTurns: int(distance.GetDistance()),

		showTroopCounter: showTroopCounter,
		ownerColor:       ownerColor,
	}

	return a
}

func (a *Army) Draw(screen *ebiten.Image, ticker int) {
	if a.deleteResource { // resouce to be deleted, no need to spend CPU cycles
		return
	}

	if a.archersCount > 0 {
		a.archers.Draw(screen, a.x-5, a.y+5, ticker)
	}
	if a.infantryCount > 0 {
		a.infantry.Draw(screen, a.x, a.y+20, ticker)
	}
	if a.knightsCount > 0 {
		a.knights.Draw(screen, a.x+10, a.y+5, ticker)
	}
	if a.showTroopCounter {
		a.drawDebugBox(screen)
	}
}
func (a *Army) Update(turn int,
	turnProgress float64,
	ticker int, // TODO(PC): can be removed after refactoring sprite to have a singleton
) error {
	if a.turnOfArrival >= turn {
		a.deleteResource = true
		return nil
	}

	a.updateArmyCoordinates(turn, turnProgress)

	return nil
}

// depending on the number of ticker per turn
// the army will bre re-drawn closer to the target position
// for the next turn
func (a *Army) updateArmyCoordinates(turn int, turnProgress float64) {
	// this should never happen
	// since the object will stop being drawn when turn == turnOfArrival
	if turn >= a.turnOfArrival {
		a.x = a.targetX
		a.y = a.targetY
		return
	}

	turnsMovedSinceStart := turn - a.turnOfCreation // E.g. distance: 10, turn of arrival: 88, starting turn: 78, current turn: 86 -> the army has moved for 8 turns

	totalDx := a.targetX - a.startX                     // total movement of the troop (from city A -> to city B)
	totalTurnDx := totalDx / float64(a.distanceInTurns) // total movement in one turn
	turnDx := turnProgress * totalTurnDx                // total movement in a tick of the the turn

	a.x = a.startX + (totalTurnDx * float64(turnsMovedSinceStart)) + turnDx

	totalDy := a.targetY - a.startY                     // total movement of the troop (from city A -> to city B)
	totalTurnDy := totalDy / float64(a.distanceInTurns) // total movement in one turn
	turnDy := turnProgress * totalTurnDy                // total movement in a tick of the the turn

	a.y = a.startY + (totalTurnDy * float64(turnsMovedSinceStart)) + turnDy
}

// ---------------------------- debug shit------------------------------------------
// AI slop
func (a *Army) drawDebugBox(screen *ebiten.Image) {
	// draw a tall narrow colored box above the army and print troop counts stacked
	boxW := 30
	boxH := 40
	box := ebiten.NewImage(boxW, boxH)
	box.Fill(a.ownerColor)

	bx := (a.x - float64(boxW)/2) + 10
	by := a.y - 44

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(bx, by)
	screen.DrawImage(box, op)

	// Print troop counts stacked vertically (one per line)
	lines := []string{
		fmt.Sprintf("A:%d", a.archersCount),
		fmt.Sprintf("I:%d", a.infantryCount),
		fmt.Sprintf("K:%d", a.knightsCount),
	}
	// small padding inside the box, and vertical spacing
	paddingX := 6
	paddingY := 1
	lineHeight := 12
	for i, l := range lines {
		ebitenutil.DebugPrintAt(screen, l, int(bx)+paddingX, int(by)+paddingY+(i*lineHeight))
	}
}

func (a *Army) Debug() {
	log.Printf("-------------------->Army %d archers, %d infantry, %d knights\n",
		a.archersCount, a.infantryCount, a.knightsCount)
}

func (a *Army) DebugCordinate() {
	log.Printf("-------------------->Army coordinates x: %f y, %f\n",
		a.x, a.y)
}
