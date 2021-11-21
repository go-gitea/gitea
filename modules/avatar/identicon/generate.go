package identicon

import (
	"fmt"
	"image"
	"image/color"
)

// RandomImageSize generates and returns a random avatar image unique to input data
// in custom size (height and width).
func RandomImageSize(size int, data []byte) (image.Image, error) {
	// we use white as background, and use dark colors to draw blocks
	imgMaker, err := New(size, color.White, DarkColors...)
	if err != nil {
		return nil, fmt.Errorf("identicon.New: %v", err)
	}
	return imgMaker.Make(data), nil
}
