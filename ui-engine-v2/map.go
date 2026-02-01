package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"log"

	_ "image/png"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"

	"github.com/alexschoenwitz/mask-invaders/ui-engine-v2/resources"
)

var (
	backgroundSprite *Sprite
	castleSprite     *Sprite
)

func init() {
	backgroundImage, _, err := image.Decode(bytes.NewReader(resources.Background_png))
	if err != nil {
		log.Fatalf("load bg image: %s", err.Error())
	}
	castleImage, _, err := image.Decode(bytes.NewReader(resources.Castle_png))
	if err != nil {
		log.Fatalf("load castle image: %s", err.Error())
	}

	frameWidth := backgroundImage.Bounds().Dx()
	// Since we're cropping 10% from each edge, the effective frame size is 80% of original
	effectiveFrameSize := float64(frameWidth/4) * 0.8 // 4 columns, 80% after 10% crop on each side
	scale := float64(screenWidth) / effectiveFrameSize

	backgroundSprite = newSprite(ebiten.NewImageFromImage(backgroundImage), 4, 1, scale, scale, 2, 0.1) // 10% crop, square scaling
	castleSprite = newSprite(ebiten.NewImageFromImage(castleImage), 4, 1, 0.2, 0.2, 2, 0.0)             // No crop for castles
}

func (g *Game) drawBackground(screen *ebiten.Image) {
	// Calculate dynamic scale based on current screen size
	frameWidth := backgroundSprite.image.Bounds().Dx() / backgroundSprite.spriteColumns
	effectiveFrameSize := float64(frameWidth) * 0.8 // 80% after 10% crop on each side
	scale := float64(g.screenWidth) / effectiveFrameSize

	// Change background every 5 turns
	// Pass turn multiplied by speed so that sprite division shows all frames
	gameTick := (int(g.currentTurn) / 5) * backgroundSprite.speed

	// Draw current frame at full opacity
	img, op := backgroundSprite.selectFrameWithScale(gameTick, 0, 0, scale, scale, 1, 1, 1, 1)
	screen.DrawImage(img, op)
}

func (g *Game) drawCity(screen *ebiten.Image, city *CityDisplay, scale float64) {
	x, y := city.X*scale, city.Y*scale

	// Get player color for the shadow marker
	playerColor := g.playerColors[city.Player]

	// Draw castle sprite without tint
	castleScale := 0.5 * scale
	// Change castle animation every 5 turns
	// Pass turn multiplied by speed so that sprite division shows all frames
	gameTick := (int(g.currentTurn) / 5) * castleSprite.speed
	img, op := castleSprite.selectFrameWithScale(gameTick, x-float64(castleSprite.frameWidth)*castleScale/2, y-float64(castleSprite.frameHeight)*castleScale/2, castleScale, castleScale, 1.0, 1.0, 1.0, 1.0)
	screen.DrawImage(img, op)

	// Draw oval shadow marker below castle (after castle for proper layering)
	// Make color less strong (30% opacity) to match troop shadows
	shadowColor := color.RGBA{
		R: playerColor.R,
		G: playerColor.G,
		B: playerColor.B,
		A: 76, // 30% of 255
	}
	// Scale shadow size with castle scale
	ovalCenterX := float32(x)
	ovalCenterY := float32(y + float64(castleSprite.frameHeight)*castleScale*0.6) // Position below castle
	ovalRadiusX := float32(float64(castleSprite.frameWidth) * castleScale * 0.6)  // Scale with castle width
	ovalRadiusY := float32(float64(castleSprite.frameWidth) * castleScale * 0.2)  // Smaller height for oval shape
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
