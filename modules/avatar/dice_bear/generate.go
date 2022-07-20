// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dice_bear

import (
	"fmt"
	"image"
	"image/draw"
	"strings"

	"codeberg.org/Codeberg/avatars"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// DiceBear is used to generate pseudo-random avatars
type DiceBear struct{}

func (_ DiceBear) Name() string {
	return "dicebear"
}

func (_ DiceBear) RandomUserImage(size int, data []byte) (image.Image, error) {
	return randomImageSize(size, data)
}

func (_ DiceBear) RandomOrgImage(size int, data []byte) (image.Image, error) {
	size = size / 2
	space := size / 20
	img := image.NewRGBA(image.Rect(0, 0, size*2, size*2))

	for i := 0; i < 4; i++ {
		av, err := randomImageSize(size, []byte(fmt.Sprintf("%s-%d", string(data), i)))
		if err != nil {
			return nil, err
		}
		pos := image.Rect((i-int(i/2)*2)*(size+space), int(i/2)*(size+space), ((i-int(i/2)*2)+1)*(size+space), (int(i/2)+1)*(size+space))
		draw.Draw(img, pos, av, image.Point{}, draw.Over)
	}

	return img, nil
}

func randomImageSize(size int, data []byte) (image.Image, error) {
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
