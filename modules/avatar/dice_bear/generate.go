// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dice_bear

import (
	"image"
	"strings"

	"codeberg.org/Codeberg/avatars"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// Identicon is used to generate pseudo-random avatars
type DiceBear struct{}

func (_ DiceBear) Name() string {
	return "dicebear"
}

func (_ DiceBear) RandomUserImage(size int, data []byte) (image.Image, error) {
	return randomImageSize(size, data)
}

func (_ DiceBear) RandomOrgImage(size int, data []byte) (image.Image, error) {
	// TODO: group 4 images to one
	return randomImageSize(size, data)
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
