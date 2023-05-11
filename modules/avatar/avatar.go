// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"

	_ "image/gif"  // for processing gif images
	_ "image/jpeg" // for processing jpeg images

	"code.gitea.io/gitea/modules/avatar/identicon"
	"code.gitea.io/gitea/modules/setting"

	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"

	_ "golang.org/x/image/webp" // for processing webp images
)

// DefaultAvatarSize is used for avatar generation, usually the avatar image saved in server won't be larger than this value.
// Unless the original file is smaller than the resized image.
const DefaultAvatarSize = 256

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
	return RandomImageSize(DefaultAvatarSize, data)
}

func resizeAvatar(data []byte) (image.Image, error) {
	imgCfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image.DecodeConfig: %w", err)
	}
	if imgCfg.Width > setting.Avatar.MaxWidth {
		return nil, fmt.Errorf("image width is too large: %d > %d", imgCfg.Width, setting.Avatar.MaxWidth)
	}
	if imgCfg.Height > setting.Avatar.MaxHeight {
		return nil, fmt.Errorf("image height is too large: %d > %d", imgCfg.Height, setting.Avatar.MaxHeight)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image.Decode: %w", err)
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
			Anchor: image.Point{X: ax, Y: ay},
		})
		if err != nil {
			return nil, err
		}
	}

	img = resize.Resize(DefaultAvatarSize, DefaultAvatarSize, img, resize.Bilinear)
	return img, nil
}

func TryToResizeAvatar(data []byte, maxOriginSize int64) ([]byte, error) {
	img, err := resizeAvatar(data)
	if err != nil {
		return nil, err
	}
	bs := bytes.Buffer{}
	if err = png.Encode(&bs, img); err != nil {
		return nil, err
	}
	resized := bs.Bytes()
	if len(data) <= int(maxOriginSize) || len(data) <= len(resized) {
		return data, nil
	}
	return resized, nil
}
