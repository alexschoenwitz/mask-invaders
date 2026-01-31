package resources

import (
	_ "embed"
)

var (
	//go:embed background.png
	Background_png []byte

	//go:embed castle.png
	Castle_png []byte

	//go:embed archer.png
	Archer_png []byte

	//go:embed knight.png
	Knight_png []byte

	//go:embed infantry.png
	Infantry_png []byte
)
