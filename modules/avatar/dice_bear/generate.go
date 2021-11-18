package dice_bear

import (
	"image"
	"strings"

	"codeberg.org/Codeberg/avatars"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

func RandomImageSize(size int, data []byte) (image.Image, error) {
	avatar := avatars.MakeAvatar(string(data))
	return svg2image(avatar, size, size)
}

func svg2image(svg string, width, height int) (image.Image, error) {
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg))
	if err != nil {
		return nil, err
	}

	icon.SetTarget(0, 0, float64(width), float64(height))
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	icon.Draw(rasterx.NewDasher(width, height, rasterx.NewScannerGV(width, height, rgba, rgba.Bounds())), 1)

	return rgba, nil
}
