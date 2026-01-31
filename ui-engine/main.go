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

	t := NewTroop(game, Gopher, 100, 100, 43, 55)
	t2 := NewTroop(game, Gopher, 150, 150, 550, 340)
	t3 := NewTroop(game, Archer, 200, 200, 630, 620)
	t4 := NewTroop(game, Archer, 300, 300, 630, 620)
	t5 := NewTroop(game, Archer, 290, 300, 630, 620)
	t6 := NewTroop(game, Archer, 277, 300, 630, 620)

	c1 := NewCastle(game, 43, 55)
	c2 := NewCastle(game, 550, 340)
	c3 := NewCastle(game, 630, 620)

	game.objects = append(game.objects, t, t2, t3, c1, c2, c3, t4, t5, t6)

	ebiten.SetTPS(30)
	ebiten.SetWindowSize(game.screenWidth, game.screenHeight)
	ebiten.SetWindowTitle("Mask invaders")

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
