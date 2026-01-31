package main

import (
	_ "image/png"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	screenWidth := 1024
	screenHeight := 768

	game := &Game{
		screenWidth:  screenWidth,
		screenHeight: screenHeight,
		tickCounter:  0,
		objects:      []gameObjects{},
	}

	t := NewTroop(game, Gopher, 100, 100)

	game.objects = append(game.objects, t)

	ebiten.SetTPS(1)
	ebiten.SetWindowSize(game.screenWidth, game.screenHeight)
	ebiten.SetWindowTitle("Mask invaders")

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
