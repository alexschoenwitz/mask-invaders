package main

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

type Sprite struct {
	image *ebiten.Image

	spriteRows    int // Number of rows
	spriteColumns int // Number of pictures per row

	frameHeight int
	frameWidth  int

	scaleX      float64
	scaleY      float64
	speed       int
	cropPercent float64 // Percentage to crop from each edge (0.1 = 10%)
}

func newSprite(
	image *ebiten.Image,
	spriteColumns, spriteRows int,
	scaleX, scaleY float64,
	speed int,
	cropPercent float64,
) *Sprite {
	iW, iH := image.Bounds().Dx(), image.Bounds().Dy()
	return &Sprite{
		image:         image,
		spriteRows:    spriteRows,
		spriteColumns: spriteColumns,
		frameWidth:    iW / spriteColumns,
		frameHeight:   iH / spriteRows,
		scaleX:        scaleX,
		scaleY:        scaleY,
		speed:         speed,
		cropPercent:   cropPercent,
	}
}

func (s *Sprite) selectFrameWithScale(gameTick int, x, y, scaleX, scaleY, tintR, tintG, tintB, tintA float64) (*ebiten.Image, *ebiten.DrawImageOptions) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scaleX, scaleY)

	// Apply color tint
	op.ColorScale.Scale(float32(tintR), float32(tintG), float32(tintB), float32(tintA))

	op.GeoM.Translate(x, y)

	// gives the frame number
	// depending on the number of frames per row and number of columns
	// example: a 5 by 3 sprite will have 15 frames in total, 5 frames per row
	frameNumber := (gameTick / minSpeed(s.speed)) % (s.spriteRows * s.spriteColumns)

	// we then find the row where the frame belongs to
	// we will use this to know how to crop the right frame from the sprite
	frameRow := frameNumber / s.spriteColumns
	frameColumn := frameNumber % s.spriteColumns

	sx, sy := frameColumn*s.frameWidth, frameRow*s.frameHeight

	// Apply cropping to remove edges
	cropX := int(float64(s.frameWidth) * s.cropPercent)
	cropY := int(float64(s.frameHeight) * s.cropPercent)

	// Draw the specific frame "slice" with cropped edges
	return s.image.SubImage(image.Rect(sx+cropX, sy+cropY, sx+s.frameWidth-cropX, sy+s.frameHeight-cropY)).(*ebiten.Image), op
}

func minSpeed(speed int) int {
	if speed < 1 {
		return 1
	}

	return speed
}
