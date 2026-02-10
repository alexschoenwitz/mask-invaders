package main

import (
	"bytes"
	"image"
	_ "image/png"
	"log"

	"github.com/alexschoenwitz/mask-invaders/ui-engine/resources"

	"github.com/hajimehoshi/ebiten/v2"
	// Needed for loading files
)

var (
	mapImage *ebiten.Image
)

// loads images before starting
// always re-use image, don't load new ones
func init() {
	img, _, err := image.Decode(bytes.NewReader(resources.Map_png))
	if err != nil {
		log.Fatal(err)
	}
	mapImage = ebiten.NewImageFromImage(img)
}

type Map struct {
	game         *Game
	screenWidth  int
	screenHeight int

	scaleX float64
	scaleY float64
}

func NewMap(screenWidth, screenHeight int) *Map {
	imgWidth := float64(mapImage.Bounds().Dx())
	imgHeight := float64(mapImage.Bounds().Dy())

	m := &Map{
		screenWidth:  screenWidth,
		screenHeight: screenHeight,
		// Calculate and store the scales here
		scaleX: float64(screenWidth) / imgWidth,
		scaleY: float64(screenHeight) / imgHeight,
	}

	return m
}
func (m *Map) Update() error {
	return nil
}

func (m *Map) Draw(screen *ebiten.Image, tickCounter int) { // Create draw options
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(m.scaleX, m.scaleY)
	screen.DrawImage(mapImage, op)
}
