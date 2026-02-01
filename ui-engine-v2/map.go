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

	// Draw simple castle icon (square with crenellations)
	castleSize := float32(20 * scale)

	// Main castle body
	vector.DrawFilledRect(screen, float32(x)-castleSize/2, float32(y)-castleSize/2, castleSize, castleSize, playerColor, false)

	// Castle border/outline
	borderColor := color.RGBA{255, 255, 255, 150}
	vector.StrokeRect(screen, float32(x)-castleSize/2, float32(y)-castleSize/2, castleSize, castleSize, 2, borderColor, false)

	// Simple crenellations on top (should sit on top edge of castle, not float above)
	crenelSize := castleSize / 5
	for i := float32(0); i < 5; i++ {
		if int(i)%2 == 0 {
			vector.DrawFilledRect(screen,
				float32(x)-castleSize/2+i*crenelSize,
				float32(y)-castleSize/2,
				crenelSize, crenelSize/2,
				playerColor, false)
		}
	}

	// Draw oval shadow marker below castle
	shadowColor := color.RGBA{
		R: playerColor.R,
		G: playerColor.G,
		B: playerColor.B,
		A: 76, // 30% of 255
	}
	ovalCenterX := float32(x)
	ovalCenterY := float32(y) + castleSize*0.8
	ovalRadiusX := castleSize * 0.6
	ovalRadiusY := castleSize * 0.2
	drawFilledOval(screen, ovalCenterX, ovalCenterY, ovalRadiusX, ovalRadiusY, shadowColor)

	// Draw city name above the castle
	ebitenutil.DebugPrintAt(screen, city.Name, int(x-30*scale), int(y-40*scale))

	// Draw troop counts next to the castle
	troopTypes := []string{"A", "B", "C"}
	yOffset := int(y) - int(20*scale)
	for _, troopType := range troopTypes {
		count := city.Troops[troopType]
		text := fmt.Sprintf("%s:%d", troopType, count)
		ebitenutil.DebugPrintAt(screen, text, int(x+25*scale), yOffset)
		yOffset += int(12 * scale)
	}

	// Draw troops around the city
	g.drawTroopsAtCity(screen, city, scale)
}
