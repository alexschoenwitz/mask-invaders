package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"log"

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
	op            *ebiten.DrawImageOptions
	numerOfFrames int
	frameCounter  int

	width, height int

	//arriveTurn int
	pos_x, pos_y float64
	alive        bool
}

func NewTroop(g *Game, t troopType, x, y float64) *Troop {
	var troop *Troop
	switch t {
	case Gopher:
		w, h := gopherImage.Bounds().Dx(), gopherImage.Bounds().Dy()
		troop = &Troop{
			image:         gopherImage,
			op:            &ebiten.DrawImageOptions{},
			numerOfFrames: 1,
			pos_x:         x,
			pos_y:         y,
			alive:         true,
			width:         w,
			height:        h,
			game:          g,
		}
	case Archer:
		w, h := archerImage.Bounds().Dx(), archerImage.Bounds().Dy()
		troop = &Troop{
			image:         archerImage,
			op:            &ebiten.DrawImageOptions{},
			numerOfFrames: 8,
			pos_x:         x,
			pos_y:         y,
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
	t.op.GeoM.Translate(-float64(t.width)/2, -float64(t.height)/2)
	t.op.GeoM.Translate(20, float64(t.game.screenHeight/2))

	// Calculate which frame to show
	i := t.game.tickCounter % t.numerOfFrames //consider making slower by diving the counter
	fmt.Println("Some: ", i)
	sx, sy := 50, 50

	// Draw the specific frame "slice"
	sub := t.image.SubImage(image.Rect(sx, sy, sx+t.width, sy+t.height)).(*ebiten.Image)
	screen.DrawImage(sub, t.op)
}

func (t *Troop) Update( /*add game*/ ) error {
	return nil
}
