package main

import (
	"bytes"
	"fmt"
	"image"
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
		log.Fatalf("load bg image: %w", err)
	}
	castleImage, _, err := image.Decode(bytes.NewReader(resources.Castle_png))
	if err != nil {
		log.Fatalf("load castle image: %w", err)
	}

	frameWidth := backgroundImage.Bounds().Dx()
	// Since we're cropping 10% from each edge, the effective frame size is 80% of original
	effectiveFrameSize := float64(frameWidth/4) * 0.8 // 4 columns, 80% after 10% crop on each side
	scale := float64(screenWidth) / effectiveFrameSize

	backgroundSprite = newSprite(ebiten.NewImageFromImage(backgroundImage), 4, 1, scale, scale, 2, 0.1) // 10% crop, square scaling
	castleSprite = newSprite(ebiten.NewImageFromImage(castleImage), 4, 1, 2, 2, 2, 0.0)                 // No crop for castles
}

func (g *Game) drawBackground(screen *ebiten.Image) {
	// Calculate dynamic scale based on current screen size
	frameWidth := backgroundSprite.image.Bounds().Dx() / backgroundSprite.spriteColumns
	effectiveFrameSize := float64(frameWidth) * 0.8 // 80% after 10% crop on each side
	scale := float64(g.screenWidth) / effectiveFrameSize
	
	totalFrames := backgroundSprite.spriteRows * backgroundSprite.spriteColumns
	currentFrame := int(g.currentTurn) % totalFrames
	
	// Draw current frame at full opacity
	img, op := backgroundSprite.selectFrameWithScale(currentFrame, 0, 0, scale, scale, 1, 1, 1, 1)
	screen.DrawImage(img, op)
}

func (g *Game) drawCity(screen *ebiten.Image, city *CityDisplay, scale float64) {
	x, y := city.X*scale, city.Y*scale

	// Get player color for tinting the castle
	playerColor := g.playerColors[city.Player]
	tintR := float64(playerColor.R) / 255.0
	tintG := float64(playerColor.G) / 255.0
	tintB := float64(playerColor.B) / 255.0
	tintA := float64(playerColor.A) / 255.0

	// Draw castle sprite at city position with player color tint and scale
	castleScale := 2.0 * scale // Castles are drawn at 2x base size
	img, op := castleSprite.selectFrameWithScale(int(g.currentTurn), x-float64(castleSprite.frameWidth)*castleScale/2, y-float64(castleSprite.frameHeight)*castleScale/2, castleScale, castleScale, tintR, tintG, tintB, tintA)
	screen.DrawImage(img, op)

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
