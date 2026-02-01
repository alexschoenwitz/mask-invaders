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
	
	// Draw celestial bodies (sun/moon)
	g.drawCelestialBodies(screen, timeOfDay, horizonY)
	
	// Draw ground (bottom 5/6 of screen)
	vector.DrawFilledRect(screen, 0, horizonY, float32(g.screenWidth), float32(g.screenHeight)-horizonY, groundColor, false)
}

func (g *Game) drawCelestialBodies(screen *ebiten.Image, timeOfDay float64, horizonY float32) {
	// Calculate position along an arc across the sky
	// timeOfDay ranges from 0 to 1
	// Sun is visible from 0.2 to 0.8 (sunrise to sunset)
	// Moon is visible from 0.8 to 0.2 next day (sunset to sunrise)
	
	skyWidth := float32(g.screenWidth)
	skyHeight := horizonY
	
	// Draw sun during daytime (0.2 to 0.8)
	if timeOfDay >= 0.2 && timeOfDay <= 0.8 {
		// Map time 0.2-0.8 to position 0-1 across the sky
		sunProgress := (timeOfDay - 0.2) / 0.6
		
		// Arc across the sky
		sunX := skyWidth * 0.1 + skyWidth*0.8*float32(sunProgress)
		sunY := skyHeight*0.9 - float32(math.Sin(sunProgress*math.Pi))*skyHeight*0.7
		
		g.drawSun(screen, sunX, sunY, 12)
	}
	
	// Draw moon during nighttime (0.8 to 1.0 and 0.0 to 0.2)
	// Night duration is 0.4 total (0.2 + 0.2)
	if timeOfDay >= 0.8 || timeOfDay <= 0.2 {
		var moonProgress float64
		if timeOfDay >= 0.8 {
			// Evening: 0.8 to 1.0 maps to 0.0 to 0.5
			moonProgress = (timeOfDay - 0.8) / 0.4
		} else {
			// Morning: 0.0 to 0.2 maps to 0.5 to 1.0
			moonProgress = (timeOfDay + 0.2) / 0.4
		}
		
		// Arc across the sky
		moonX := skyWidth * 0.1 + skyWidth*0.8*float32(moonProgress)
		moonY := skyHeight*0.9 - float32(math.Sin(moonProgress*math.Pi))*skyHeight*0.7
		
		g.drawMoon(screen, moonX, moonY, 10)
	}
}

func (g *Game) drawSun(screen *ebiten.Image, x, y, radius float32) {
	sunColor := color.RGBA{255, 220, 0, 255}
	
	// Draw central circle
	vector.DrawFilledCircle(screen, x, y, radius, sunColor, false)
	
	// Draw 8 rays
	rayLength := radius * 1.8
	
	// 4 cardinal directions (longer)
	cardinalLength := rayLength * 1.3
	// North
	vector.StrokeLine(screen, x, y-radius, x, y-radius-cardinalLength, 2, sunColor, false)
	// South
	vector.StrokeLine(screen, x, y+radius, x, y+radius+cardinalLength, 2, sunColor, false)
	// East
	vector.StrokeLine(screen, x+radius, y, x+radius+cardinalLength, y, 2, sunColor, false)
	// West
	vector.StrokeLine(screen, x-radius, y, x-radius-cardinalLength, y, 2, sunColor, false)
	
	// 4 diagonal directions (shorter)
	offset := radius * 0.707 // cos(45°) or sin(45°)
	diagLength := rayLength
	diagOffset := diagLength * 0.707
	
	// NE
	vector.StrokeLine(screen, x+offset, y-offset, x+offset+diagOffset, y-offset-diagOffset, 2, sunColor, false)
	// SE
	vector.StrokeLine(screen, x+offset, y+offset, x+offset+diagOffset, y+offset+diagOffset, 2, sunColor, false)
	// SW
	vector.StrokeLine(screen, x-offset, y+offset, x-offset-diagOffset, y+offset+diagOffset, 2, sunColor, false)
	// NW
	vector.StrokeLine(screen, x-offset, y-offset, x-offset-diagOffset, y-offset-diagOffset, 2, sunColor, false)
}

func (g *Game) drawMoon(screen *ebiten.Image, x, y, radius float32) {
	moonColor := color.RGBA{220, 220, 240, 255}
	
	// Draw crescent moon (simple circle for now)
	vector.DrawFilledCircle(screen, x, y, radius, moonColor, false)
	
	// Add a darker crescent shadow for moon phase effect
	shadowColor := color.RGBA{180, 180, 200, 255}
	shadowOffset := radius * 0.3
	vector.DrawFilledCircle(screen, x+shadowOffset, y, radius*0.8, shadowColor, false)
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
