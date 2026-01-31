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
	gopherImage *ebiten.Image
	archerImage *ebiten.Image
	knightImage *ebiten.Image
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
	iW, iH := image.Bounds().Dx(), archerImage.Bounds().Dy()
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

	op.GeoM.Translate(-float64(s.frameWidth)/2, -float64(s.frameHeight)/2)
	op.GeoM.Translate(x, y)

	// gives the frame number
	// depending on the number of frames per row and number of columns
	// example: a 5 by 3 sprite will have 15 frames in total, 5 frames per row
	frameNumber := (gameTick / minSpeed(s.speed)) % (s.spriteRows * s.spriteColumns)

	// we then find the row where the frame belongs to
	// we will use this to know how to crop the right frame from the sprite
	frameRow := frameNumber / s.spriteColumns
	frameColumn := frameNumber % s.spriteRows

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

type Troop struct {
	game   *Game
	sprite *Sprite

	//arriveTurn int
	startX, startY   float64
	targetX, targetY float64
	x, y             float64
	alive            bool
}

func NewTroop(g *Game, t troopType, sX, sY float64, tX, tY float64) *Troop {
	var troop *Troop
	switch t {
	case Gopher:
		troop = &Troop{
			game:    g,
			sprite:  newSprite(gopherImage, 1, 1, 2, 2, 1),
			startX:  sX,
			startY:  sY,
			x:       sX,
			y:       sY,
			targetX: tX,
			targetY: tY,
			alive:   true,
		}
	case Archer:
		troop = &Troop{
			game:    g,
			sprite:  newSprite(archerImage, 1, 8, 1, 1, 1),
			startX:  sX,
			startY:  sY,
			x:       sX,
			y:       sY,
			targetX: tX,
			targetY: tY,
			alive:   true,
		}
	case Knight:
		troop = &Troop{
			game:    g,
			sprite:  newSprite(knightImage, 4, 8, 1, 1, 1),
			startX:  sX,
			startY:  sY,
			x:       sX,
			y:       sY,
			targetX: tX,
			targetY: tY,
			alive:   true,
		}
	case CastleTmp:
		troop = &Troop{
			game:    g,
			sprite:  newSprite(castleImage, 4, 3, 0.3, 0.3, 2),
			startX:  sX,
			startY:  sY,
			x:       sX,
			y:       sY,
			targetX: tX,
			targetY: tY,
			alive:   true,
		}
	}

	return troop
}

func (t *Troop) Draw(screen *ebiten.Image) {
	screen.DrawImage(t.sprite.selectFrame(t.game.tickCounter, t.x, t.y))
}

func (t *Troop) Update( /*add game*/ ) error {
	t.moveTo(t.targetX, t.targetY)
	return nil
}

func (t *Troop) moveTo(targetX, targetY float64) {
	dx := targetX - t.x
	dy := targetY - t.y

	distance := math.Sqrt(dx*dx + dy*dy)

	if distance > 1 {
		t.x += (dx / distance) * 1
		t.y += (dy / distance) * 1
	}
}
