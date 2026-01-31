package resources

import (
	_ "embed"
)

var (
	//go:embed background.png
	Background_png []byte

	//go:embed castle.png
	Castle_png []byte
)
