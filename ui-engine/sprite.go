package main

import (
	"image"
	_ "image/png"

	"github.com/hajimehoshi/ebiten/v2"
)

type Sprite struct {
	image *ebiten.Image

	spriteRows    int // Number of rows
	spriteColumns int // Number of pictures per row

	frameHeight int
	frameWidth  int

	scaleX float64
	scaleY float64
	speed  int
}

func newSprite(
	image *ebiten.Image,
	spriteColumns, spriteRows int,
	scaleX, scaleY float64,
	speed int,
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
	}
}

func (s *Sprite) selectFrame(gameTick int, x, y float64) (*ebiten.Image, *ebiten.DrawImageOptions) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s.scaleX, s.scaleY)
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

	// Draw the specific frame "slice"
	return s.image.SubImage(image.Rect(sx, sy, sx+s.frameWidth, sy+s.frameHeight)).(*ebiten.Image), op

}

func minSpeed(speed int) int {
	if speed < 1 {
		return 1
	}

	return speed
}
