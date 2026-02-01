package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type TroopSprite struct {
	troopType string
}

func NewTroopSprite(troopType string) *TroopSprite {
	return &TroopSprite{
		troopType: troopType,
	}
}

func (t *TroopSprite) Draw(screen *ebiten.Image, gameTick int, x, y, scale float64, r, g, b, a float64) {
	size := float32(8 * scale)
	clr := color.RGBA{
		R: uint8(r * 255),
		G: uint8(g * 255),
		B: uint8(b * 255),
		A: uint8(a * 255),
	}

	switch t.troopType {
	case "A": // Archer - Triangle
		vector.DrawFilledRect(screen, float32(x)-size/2, float32(y)-size/2, size, size, clr, false)
	case "B": // Knight - Square
		vector.DrawFilledCircle(screen, float32(x), float32(y), size/2, clr, false)
	case "C": // Infantry - Circle
		// Draw triangle
		x1, y1 := float32(x), float32(y)-size/2
		x2, y2 := float32(x)-size/2, float32(y)+size/2
		x3, y3 := float32(x)+size/2, float32(y)+size/2
		vector.StrokeLine(screen, x1, y1, x2, y2, 2, clr, false)
		vector.StrokeLine(screen, x2, y2, x3, y3, 2, clr, false)
		vector.StrokeLine(screen, x3, y3, x1, y1, 2, clr, false)
	default:
		vector.DrawFilledCircle(screen, float32(x), float32(y), size/2, clr, false)
	}
}
