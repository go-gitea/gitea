// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package identicon

import (
	"fmt"
	"image"
	"image/color"
)

// Identicon is used to generate pseudo-random avatars
type Identicon struct{}

func (Identicon) Name() string {
	return "identicon"
}

func (Identicon) RandomUserImage(size int, data []byte) (image.Image, error) {
	return randomImageSize(size, data)
}

func (Identicon) RandomOrgImage(size int, data []byte) (image.Image, error) {
	return randomImageSize(size, data)
}

func (Identicon) RandomRepoImage(size int, data []byte) (image.Image, error) {
	return randomImageSize(size, data)
}

// randomImageSize generates and returns a random avatar image unique to input data
// in custom size (height and width).
func randomImageSize(size int, data []byte) (image.Image, error) {
	// we use white as background, and use dark colors to draw blocks
	imgMaker, err := new(size, color.White, DarkColors...)
	if err != nil {
		return nil, fmt.Errorf("identicon.New: %v", err)
	}
	return imgMaker.Make(data), nil
}
