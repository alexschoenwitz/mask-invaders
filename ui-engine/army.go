package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"log"
	"math"

	"github.com/alexschoenwitz/mask-invaders/ui-engine/resources"

	"github.com/hajimehoshi/ebiten/v2"
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

type Sprite struct {
	image *ebiten.Image

	spriteRows    int // Number of rows
	spriteColumns int // Number of pictures per row

	frameHeight int
	frameWidth  int

	scaleX float64
	scaleY float64
	speed  int
}

func newSprite(
	image *ebiten.Image,
	spriteColumns, spriteRows int,
	scaleX, scaleY float64,
	speed int,
) *Sprite {
	iW, iH := image.Bounds().Dx(), image.Bounds().Dy()
	return &Sprite{
		image:         image,
		spriteRows:    spriteRows,
		spriteColumns: spriteColumns,
		frameWidth:    iW / spriteColumns,
		frameHeight:   iH / spriteRows,
		scaleX:        scaleX,
		scaleY:        scaleY,
		speed:         speed,
	}
}

func (s *Sprite) selectFrame(gameTick int, x, y float64) (*ebiten.Image, *ebiten.DrawImageOptions) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s.scaleX, s.scaleY)
	fmt.Println("---------------------")
	op.GeoM.Translate(x, y)

	// gives the frame number
	// depending on the number of frames per row and number of columns
	// example: a 5 by 3 sprite will have 15 frames in total, 5 frames per row
	frameNumber := (gameTick / minSpeed(s.speed)) % (s.spriteRows * s.spriteColumns)

	// we then find the row where the frame belongs to
	// we will use this to know how to crop the right frame from the sprite
	frameRow := frameNumber / s.spriteColumns
	frameColumn := frameNumber % s.spriteColumns

	fmt.Println("frame R: ", frameRow)
	fmt.Println("frame C: ", frameColumn)

	sx, sy := frameColumn*s.frameWidth, frameRow*s.frameHeight

	fmt.Println("StartX: ", sx, sx+s.frameWidth)
	fmt.Println("StartY: ", sy, s.frameHeight)

	fmt.Println("---------------------")

	// Draw the specific frame "slice"
	return s.image.SubImage(image.Rect(sx, sy, sx+s.frameWidth, sy+s.frameHeight)).(*ebiten.Image), op

}

func minSpeed(speed int) int {
	if speed < 1 {
		return 1
	}

	return speed
}

type Army struct {
	game *Game

	knights      *Troop
	knightsCount int

	archers      *Troop
	archersCount int

	infantry      *Troop
	infantryCount int

	startX, startY   float64
	targetX, targetY float64
	x, y             float64

	showTroopCounter bool
}

func NewArmy(g *Game,
	sX, sY float64, tX, tY float64,
	knight, archer, infantry int,
	showTroopCounter bool,
) *Army {
	return &Army{
		game:    g,
		startX:  sX,
		startY:  sY,
		x:       sX,
		y:       sY,
		targetX: tX,
		targetY: tY,

		knightsCount:  knight,
		knights:       NewTroop(g, Knight),
		archersCount:  archer,
		archers:       NewTroop(g, Archer),
		infantryCount: infantry,
		infantry:      NewTroop(g, Infantry),

		showTroopCounter: showTroopCounter,
	}
}

func (a *Army) Draw(screen *ebiten.Image) {
	if a.archersCount > 0 {
		a.archers.Draw(screen, a.x-5, a.y+5)
	}
	if a.infantryCount > 0 {
		a.infantry.Draw(screen, a.x, a.y+20)
	}
	if a.knightsCount > 0 {
		a.knights.Draw(screen, a.x+10, a.y+5)
	}
	if a.showTroopCounter {
		// TODO add counter UI
	}
}

func (a *Army) Update() error {
	a.moveArmyToDestination()
	return nil
}

func (a *Army) moveArmyToDestination() {
	dx := a.targetX - a.x
	dy := a.targetY - a.y

	distance := math.Sqrt(dx*dx + dy*dy)

	if distance > 1 {
		a.x += (dx / distance) * 1
		a.y += (dy / distance) * 1
	}
}

type Troop struct {
	game   *Game
	sprite *Sprite
}

func NewTroop(g *Game, t troopType) *Troop {
	var troop *Troop
	switch t {
	case Gopher:
		troop = &Troop{
			game:   g,
			sprite: newSprite(gopherImage, 1, 1, 2, 2, 1),
		}
	case Archer:
		troop = &Troop{
			game:   g,
			sprite: newSprite(archerImage, 4, 2, 0.1, 0.1, 10),
		}
	case Knight:
		troop = &Troop{
			game:   g,
			sprite: newSprite(knightImage, 4, 2, .15, .15, 10),
		}
	case Infantry:
		troop = &Troop{
			game:   g,
			sprite: newSprite(infantryImage, 4, 2, 0.1, 0.1, 10),
		}
	case CastleTmp:
		troop = &Troop{
			game:   g,
			sprite: newSprite(castleImage, 4, 3, 0.3, 0.3, 2),
		}
	}

	return troop
}

func (t *Troop) Draw(screen *ebiten.Image, x, y float64) {
	screen.DrawImage(t.sprite.selectFrame(t.game.tickCounter, x, y))
}
