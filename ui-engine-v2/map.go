package main

import (
	"bytes"
	"fmt"
	"log"

	"image"
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

	frameWidth, frameHeight := backgroundImage.Bounds().Dx(), backgroundImage.Bounds().Dy()
	scaleX := float64(screenWidth) / float64(frameWidth/4)
	scaleY := float64(screenHeight) / float64(frameHeight)

	backgroundSprite = newSprite(ebiten.NewImageFromImage(backgroundImage), 4, 1, scaleX, scaleY, 2)
	castleSprite = newSprite(ebiten.NewImageFromImage(castleImage), 4, 1, 2, 2, 2)
}

func (g *Game) drawBackground(screen *ebiten.Image) {
	screen.DrawImage(backgroundSprite.selectFrame(int(g.currentTurn), 0, 0))
}

func (g *Game) drawCity(screen *ebiten.Image, city *CityDisplay) {
	x, y := city.X, city.Y

	// Draw castle sprite at city position
	screen.DrawImage(castleSprite.selectFrame(int(g.currentTurn), x-float64(castleSprite.frameWidth)/2, y-float64(castleSprite.frameHeight)/2))

	// Draw city name above the castle
	ebitenutil.DebugPrintAt(screen, city.Name, int(x-30), int(y-40))

	// Draw troop counts next to the castle
	troopTypes := []string{"A", "B", "C"}
	yOffset := int(y) - 20
	for _, troopType := range troopTypes {
		count := city.Troops[troopType]
		text := fmt.Sprintf("%s:%d", troopType, count)
		ebitenutil.DebugPrintAt(screen, text, int(x+25), yOffset)
		yOffset += 12
	}

	// Draw troops around the city
	g.drawTroopsAtCity(screen, city)
}
