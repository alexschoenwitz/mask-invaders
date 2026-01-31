package main

import (
	_ "image/png"

	"github.com/hajimehoshi/ebiten/v2"
	// Needed for loading files
)

type Game struct {
	screenWidth  int
	screenHeight int
	tickCounter  int

	objects []gameObjects
}

type gameObjects interface {
	Draw(screen *ebiten.Image)
	Update() error
}

func (g *Game) Update() error {
	g.tickCounter++
	for _, o := range g.objects {
		err := o.Update()
		if err != nil {
			return err
		}
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	for _, o := range g.objects {
		o.Draw(screen)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return g.screenWidth, g.screenHeight
}
