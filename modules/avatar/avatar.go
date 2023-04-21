// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar

import (
	"bytes"
	"fmt"
	"image"
	"image/color"

	_ "image/gif"  // for processing gif images
	_ "image/jpeg" // for processing jpeg images
	_ "image/png"  // for processing png images

	"code.gitea.io/gitea/modules/avatar/identicon"
	"code.gitea.io/gitea/modules/setting"

	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"

	_ "golang.org/x/image/webp" // for processing webp images
)

// AvatarSize returns avatar's size
const AvatarSize = 290

// RandomImageSize generates and returns a random avatar image unique to input data
// in custom size (height and width).
func RandomImageSize(size int, data []byte) (image.Image, error) {
	// we use white as background, and use dark colors to draw blocks
	imgMaker, err := identicon.New(size, color.White, identicon.DarkColors...)
	if err != nil {
		return nil, fmt.Errorf("identicon.New: %w", err)
	}
	return imgMaker.Make(data), nil
}

// RandomImage generates and returns a random avatar image unique to input data
// in default size (height and width).
func RandomImage(data []byte) (image.Image, error) {
	return RandomImageSize(AvatarSize, data)
}

// Prepare accepts a byte slice as input, validates it contains an image of an
// acceptable format, and crops and resizes it appropriately.
func Prepare(data []byte) (*image.Image, error) {
	imgCfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("DecodeConfig: %w", err)
	}
	if imgCfg.Width > setting.Avatar.MaxWidth {
		return nil, fmt.Errorf("Image width is too large: %d > %d", imgCfg.Width, setting.Avatar.MaxWidth)
	}
	if imgCfg.Height > setting.Avatar.MaxHeight {
		return nil, fmt.Errorf("Image height is too large: %d > %d", imgCfg.Height, setting.Avatar.MaxHeight)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("Decode: %w", err)
	}

	if imgCfg.Width != imgCfg.Height {
		var newSize, ax, ay int
		if imgCfg.Width > imgCfg.Height {
			newSize = imgCfg.Height
			ax = (imgCfg.Width - imgCfg.Height) / 2
		} else {
			newSize = imgCfg.Width
			ay = (imgCfg.Height - imgCfg.Width) / 2
		}

		img, err = cutter.Crop(img, cutter.Config{
			Width:  newSize,
			Height: newSize,
			Anchor: image.Point{ax, ay},
		})
		if err != nil {
			return nil, err
		}
	}

	img = resize.Resize(AvatarSize, AvatarSize, img, resize.Bilinear)
	return &img, nil
}
