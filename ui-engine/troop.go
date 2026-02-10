package main

import "github.com/hajimehoshi/ebiten/v2"

// TODO(PC): Move Troop to a new file
type Troop struct {
	sprite *Sprite
}

// TODO(PC): Don't create new sprites every time
// Create once, re-use always
func NewTroop(t troopType) *Troop {
	var troop *Troop

	switch t {
	case Gopher:
		troop = &Troop{
			sprite: newSprite(gopherImage, 1, 1, 2, 2, 1),
		}
	case Archer:
		troop = &Troop{
			sprite: newSprite(archerImage, 4, 2, 0.1, 0.1, 10),
		}
	case Knight:
		troop = &Troop{
			sprite: newSprite(knightImage, 4, 2, 0.15, 0.15, 10),
		}
	case Infantry:
		troop = &Troop{
			sprite: newSprite(infantryImage, 4, 2, 0.1, 0.1, 10),
		}
	}

	return troop
}

// TODO(PC): Selecting frame for each is expensive
// move everything to a singleton that keeps all sprite objects and a single frame selected that can be shared
// TODO(PC): Consider thread safety for paralelizing the Drawing
func (t *Troop) Draw(screen *ebiten.Image, x, y float64, tickCounter int) {
	screen.DrawImage(t.sprite.selectFrame(tickCounter, x, y))
}
