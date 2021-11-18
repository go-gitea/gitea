package identicon

import (
	"fmt"
	"image"
	"image/color/palette"

	"code.gitea.io/gitea/modules/util"

	"github.com/issue9/identicon"
)

// RandomImageSize generates and returns a random avatar image unique to input data
// in custom size (height and width).
func RandomImageSize(size int, data []byte) (image.Image, error) {
	randExtent := len(palette.WebSafe) - 32
	integer, err := util.RandomInt(int64(randExtent))
	if err != nil {
		return nil, fmt.Errorf("util.RandomInt: %v", err)
	}
	colorIndex := int(integer)
	backColorIndex := colorIndex - 1
	if backColorIndex < 0 {
		backColorIndex = randExtent - 1
	}

	// Define size, background, and forecolor
	imgMaker, err := identicon.New(size,
		palette.WebSafe[backColorIndex], palette.WebSafe[colorIndex:colorIndex+32]...)
	if err != nil {
		return nil, fmt.Errorf("identicon.New: %v", err)
	}
	return imgMaker.Make(data), nil
}
