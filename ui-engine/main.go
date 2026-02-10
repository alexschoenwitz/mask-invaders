package main

import (
	_ "image/png"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	SCREEN_WIDTH  = 1024
	SCREEN_HEIGHT = 768
	// We can define how many times the Update fuction will be called per second
	// at the same time we can define how many times the update needs to be called
	// before moving to the next turn
	// we can effectively control how many turns per second by controlling this two variables
	UPDATE_PER_SECOND = 20
	TURN_PER_SECOND   = 2
)

func main() {
	game, err := NewGame(FancyDistribution, SCREEN_WIDTH, SCREEN_HEIGHT, UPDATE_PER_SECOND/TURN_PER_SECOND)
	if err != nil {
		log.Fatal(err)
	}

	ebiten.SetTPS(UPDATE_PER_SECOND)
	ebiten.SetWindowSize(game.screenWidth, game.screenHeight)
	ebiten.SetWindowTitle("Mask invaders")

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
