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
	castleImage *ebiten.Image
)

// loads images before starting
// always re-use image, don't load new ones
func init() {
	img, _, err := image.Decode(bytes.NewReader(resources.Castle_png))
	if err != nil {
		log.Fatal(err)
	}
	castleImage = ebiten.NewImageFromImage(img)
}

type Castle struct {
	game          *Game
	image         *ebiten.Image
	numerOfFrames int
	frameCounter  int

	width, height int

	//arriveTurn int
	pos_x, pos_y float64
	alive        bool
}

func NewCastle(g *Game, x, y float64) *Castle {
	w, h := castleImage.Bounds().Dx(), castleImage.Bounds().Dy()
	return &Castle{
		image:         castleImage,
		numerOfFrames: 4,
		pos_x:         x,
		pos_y:         y,
		alive:         true,
		width:         w / 4, // divide sprite by number of frames
		height:        h,
		game:          g,
	}
}

func (t *Castle) Draw(screen *ebiten.Image) {
	fmt.Println(t.width, t.height)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(0.1, 0.1)

	op.GeoM.Translate(-float64(t.width)/2, -float64(t.height)/2)
	op.GeoM.Translate(t.pos_x, t.pos_y)

	// Calculate which frame to show
	i := (t.game.tickCounter / 50) % t.numerOfFrames //consider making slower by diving the counter
	fmt.Println("Some: ", i)
	sx, sy := i*t.width, 0

	// Draw the specific frame "slice"
	sub := t.image.SubImage(image.Rect(sx, sy, sx+t.width, sy+t.height)).(*ebiten.Image)
	screen.DrawImage(sub, op)
}

func (t *Castle) Update( /*add game*/ ) error {
	return nil
}
