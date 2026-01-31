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

	a1 := NewArmy(game, 50, 60, 733, 553, 10, 3, 5, false)

	c1 := NewCastle(game, 43, 55)
	c2 := NewCastle(game, 550, 340)
	c3 := NewCastle(game, 630, 620)

	game.objects = append(game.objects, a1, c1, c2, c3,
		NewArmy(game, 50, 60, 733, 553, 10, 3, 5, false),
		NewArmy(game, 50, 65, 733, 553, 10, 3, 5, false),
		NewArmy(game, 50, 70, 733, 553, 10, 3, 5, false),
		NewArmy(game, 51, 60, 733, 553, 10, 3, 5, false),
		NewArmy(game, 52, 60, 733, 553, 10, 3, 5, false),
		NewArmy(game, 53, 60, 733, 553, 10, 3, 5, false),
		NewArmy(game, 55, 60, 733, 553, 10, 3, 5, false),
		NewArmy(game, 58, 60, 733, 553, 10, 3, 5, false),
		NewArmy(game, 60, 60, 733, 553, 10, 3, 5, false),
		NewArmy(game, 45, 60, 733, 553, 10, 3, 5, false),
		NewArmy(game, 599, 60, 733, 553, 10, 3, 5, false),
	)

	ebiten.SetTPS(30)
	ebiten.SetWindowSize(game.screenWidth, game.screenHeight)
	ebiten.SetWindowTitle("Mask invaders")

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
