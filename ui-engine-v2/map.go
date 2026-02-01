package main

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (g *Game) drawBackground(screen *ebiten.Image) {
	// Simulate day/night cycle - one full cycle every 100 turns for slower, more noticeable transition
	timeOfDay := math.Mod(g.currentTurn, 100) / 100 // 0 to 1
	
	// More realistic day/night cycle
	var skyR, skyG, skyB uint8
	if timeOfDay < 0.2 {
		// Night (0-0.2): dark blue
		skyR, skyG, skyB = 25, 25, 50
	} else if timeOfDay < 0.35 {
		// Sunrise (0.2-0.35): dark -> orange -> bright blue
		t := (timeOfDay - 0.2) / 0.15
		if t < 0.5 {
			// Dark to orange
			t2 := t * 2
			skyR = uint8(25 + 230*t2)
			skyG = uint8(25 + 115*t2)
			skyB = uint8(50 + 30*t2)
		} else {
			// Orange to bright blue
			t2 := (t - 0.5) * 2
			skyR = uint8(255 - 120*t2)
			skyG = uint8(140 + 66*t2)
			skyB = uint8(80 + 155*t2)
		}
	} else if timeOfDay < 0.65 {
		// Day (0.35-0.65): bright sky blue
		skyR, skyG, skyB = 135, 206, 235
	} else if timeOfDay < 0.8 {
		// Sunset (0.65-0.8): bright blue -> orange -> dark
		t := (timeOfDay - 0.65) / 0.15
		if t < 0.5 {
			// Bright blue to orange
			t2 := t * 2
			skyR = uint8(135 + 120*t2)
			skyG = uint8(206 - 66*t2)
			skyB = uint8(235 - 155*t2)
		} else {
			// Orange to dark
			t2 := (t - 0.5) * 2
			skyR = uint8(255 - 230*t2)
			skyG = uint8(140 - 115*t2)
			skyB = uint8(80 - 30*t2)
		}
	} else {
		// Night (0.8-1.0): dark blue
		skyR, skyG, skyB = 25, 25, 50
	}
	
	skyColor := color.RGBA{skyR, skyG, skyB, 255}
	groundColor := color.RGBA{34, 139, 34, 255} // Forest green
	
	// Draw sky (top 1/6 of screen)
	horizonY := float32(g.screenHeight / 6)
	vector.DrawFilledRect(screen, 0, 0, float32(g.screenWidth), horizonY, skyColor, false)
	
	// Draw ground (bottom 5/6 of screen)
	vector.DrawFilledRect(screen, 0, horizonY, float32(g.screenWidth), float32(g.screenHeight)-horizonY, groundColor, false)
}

func (g *Game) drawCity(screen *ebiten.Image, city *CityDisplay, scale float64) {
	x, y := city.X*scale, city.Y*scale

	// Get player color for the castle
	playerColor := g.playerColors[city.Player]

	// Draw simple castle icon with prominent crenellations
	castleSize := float32(20 * scale)
	
	// Main castle body (slightly darker shade of player color for depth)
	bodyColor := color.RGBA{
		R: uint8(float32(playerColor.R) * 0.9),
		G: uint8(float32(playerColor.G) * 0.9),
		B: uint8(float32(playerColor.B) * 0.9),
		A: 255,
	}
	vector.DrawFilledRect(screen, float32(x)-castleSize/2, float32(y)-castleSize/4, castleSize, castleSize*0.75, bodyColor, false)

	// Three prominent crenellations (towers) on top
	towerWidth := castleSize / 3.5
	towerHeight := castleSize / 2
	
	// Left tower
	vector.DrawFilledRect(screen, float32(x)-castleSize/2, float32(y)-castleSize/2, towerWidth, towerHeight, playerColor, false)
	// Middle tower (slightly taller)
	vector.DrawFilledRect(screen, float32(x)-towerWidth/2, float32(y)-castleSize/2-castleSize*0.15, towerWidth, towerHeight+castleSize*0.15, playerColor, false)
	// Right tower
	vector.DrawFilledRect(screen, float32(x)+castleSize/2-towerWidth, float32(y)-castleSize/2, towerWidth, towerHeight, playerColor, false)
	
	// Small gate/door at bottom (darker)
	gateColor := color.RGBA{0, 0, 0, 100}
	gateWidth := castleSize / 3
	gateHeight := castleSize / 3
	vector.DrawFilledRect(screen, float32(x)-gateWidth/2, float32(y)+castleSize/4-gateHeight, gateWidth, gateHeight, gateColor, false)

	// Don't draw city name - just show the colored castle and troop counts
	
	// Draw troop counts in compact single line format: "5/10/3" centered below castle
	troopText := fmt.Sprintf("%d/%d/%d", 
		city.Troops["A"], 
		city.Troops["B"], 
		city.Troops["C"])
	// Center the text below castle (assuming ~6 pixels per char, 7-8 chars average)
	textOffset := 20.0 // Rough half-width of text
	ebitenutil.DebugPrintAt(screen, troopText, int(x-textOffset), int(y+float64(castleSize)*0.6))
}
