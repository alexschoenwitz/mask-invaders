package main

import (
	"bytes"
	"image"
	_ "image/png"
	"log"

	"github.com/alexschoenwitz/mask-invaders/ui-engine-v2/resources"
	"github.com/hajimehoshi/ebiten/v2"
)

var (
	archerImage   *ebiten.Image
	knightImage   *ebiten.Image
	infantryImage *ebiten.Image
)

func init() {
	img, _, err := image.Decode(bytes.NewReader(resources.Archer_png))
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
}

type TroopSprite struct {
	sprite *Sprite
}

func NewTroopSprite(troopType string) *TroopSprite {
	var img *ebiten.Image
	switch troopType {
	case "A":
		img = archerImage
	case "B":
		img = knightImage
	case "C":
		img = infantryImage
	default:
		img = infantryImage
	}

	return &TroopSprite{
		sprite: newSprite(img, 4, 2, 0.1, 0.1, 10, 0.0),
	}
}

func (t *TroopSprite) Draw(screen *ebiten.Image, gameTick int, x, y, scale float64, r, g, b, a float64) {
	// Apply scale to the sprite
	scaledSpriteScale := t.sprite.scaleX * scale
	img, op := t.sprite.selectFrameWithScale(gameTick, x, y, scaledSpriteScale, scaledSpriteScale, r, g, b, a)
	screen.DrawImage(img, op)
}
