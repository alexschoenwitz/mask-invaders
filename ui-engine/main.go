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

	t := NewTroop(game, CastleTmp, 100, 100, 733, 553)

	c1 := NewCastle(game, 43, 55)
	c2 := NewCastle(game, 550, 340)
	c3 := NewCastle(game, 630, 620)

	game.objects = append(game.objects, t, c1, c2, c3)

	ebiten.SetTPS(30)
	ebiten.SetWindowSize(game.screenWidth, game.screenHeight)
	ebiten.SetWindowTitle("Mask invaders")

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
