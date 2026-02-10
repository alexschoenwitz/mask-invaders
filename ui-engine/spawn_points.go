package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"log"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"github.com/alexschoenwitz/mask-invaders/ui-engine/resources"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	castleScaleX float64 = 0.15
	castleScaleY float64 = 0.15
)

var (
	castleImage *ebiten.Image
)

// loads images before starting
// always re-use image, don't load new ones
func init() {
	img, _, err := image.Decode(bytes.NewReader(resources.Castle_png))
	if err != nil {
		log.Fatal(err)
	}
	castleImage = ebiten.NewImageFromImage(img)
}

type SpawnPoint struct {
	P  Point  // will never change during the game
	Id string // will never change during the game

	sprite *Sprite
	state  *api.City
}

func NewSpawnPoint(p Point, id string, gameState *api.State) (*SpawnPoint, error) {
	s := newSprite(castleImage, 1, 1, castleScaleX, castleScaleY, 10)

	if id == "" {
		return nil, fmt.Errorf("missing id for creating SpawnPoint")
	}
	if gameState == nil {
		return nil, fmt.Errorf("missing game state for creating SpawnPoint")
	}

	sp := &SpawnPoint{
		sprite: s,
		P:      p,
		Id:     id,
	}

	err := sp.Update(gameState)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("while creating SpawnPoint: %s", id))
	}

	return sp, nil
}

func (sp *SpawnPoint) Draw(screen *ebiten.Image, tickCounter int) {
	screen.DrawImage(sp.sprite.selectFrame(tickCounter, sp.P.x, sp.P.y))
	// TODO(PC): Add box on top with troop distribution
}

// Given the updated full game state,
// the SpawnPoint knows it's own ID and is responsible for updating itself
func (sp *SpawnPoint) Update(gameState *api.State) error {
	if gameState == nil {
		return fmt.Errorf("failed to update SpawnPoint")
	}
	sp.state = gameState.Cities[sp.Id]

	return nil
}
