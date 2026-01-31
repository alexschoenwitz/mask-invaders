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
}

type troopType int

const (
	Gopher troopType = iota
	Archer
)

type Troop struct {
	game          *Game
	image         *ebiten.Image
	numerOfFrames int
	frameCounter  int

	width, height int

	//arriveTurn int
	posX, posY        float64
	targetX, target_y float64
	alive             bool
}

func NewTroop(g *Game, t troopType, x, y float64, targetX, targetY float64) *Troop {
	var troop *Troop
	switch t {
	case Gopher:
		w, h := gopherImage.Bounds().Dx(), gopherImage.Bounds().Dy()
		troop = &Troop{
			image:         gopherImage,
			numerOfFrames: 1,
			posX:          x,
			posY:          y,
			targetX:       targetX,
			target_y:      targetY,
			alive:         true,
			width:         w,
			height:        h,
			game:          g,
		}
	case Archer:
		w, h := archerImage.Bounds().Dx(), archerImage.Bounds().Dy()
		troop = &Troop{
			image:         archerImage,
			numerOfFrames: 8,
			posX:          x,
			posY:          y,
			targetX:       targetX,
			target_y:      targetY,
			alive:         true,
			width:         w / 8, // divide sprite by number of frames
			height:        h,
			game:          g,
		}
	}
	return troop
}

func (t *Troop) Draw(screen *ebiten.Image) {
	fmt.Println(t.width, t.height)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(0.7, 0.7)

	op.GeoM.Translate(-float64(t.width)/2, -float64(t.height)/2)
	op.GeoM.Translate(t.posX, t.posY)

	// Calculate which frame to show
	i := (t.game.tickCounter / t.numerOfFrames) % t.numerOfFrames //consider making slower by diving the counter
	fmt.Println("Some: ", i)
	sx, sy := i*t.width, 0

	// Draw the specific frame "slice"
	sub := t.image.SubImage(image.Rect(sx, sy, sx+t.width, sy+t.height)).(*ebiten.Image)
	screen.DrawImage(sub, op)
}

func (t *Troop) Update( /*add game*/ ) error {
	t.moveTo(t.targetX, t.target_y)
	return nil
}

func (t *Troop) moveTo(targetX, targetY float64) {
	dx := targetX - t.posX
	dy := targetY - t.posY

	distance := math.Sqrt(dx*dx + dy*dy)

	if distance > 1 {
		t.posX += (dx / distance) * 1
		t.posY += (dy / distance) * 1
	}
}
