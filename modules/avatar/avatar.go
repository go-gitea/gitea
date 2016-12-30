// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package avatar

import (
	"fmt"
	"image"
	"image/color/palette"
	"math/rand"
	"time"

	"github.com/issue9/identicon"
)

// AvatarSize returns avatar's size
const AvatarSize = 290

// RandomImageSize generates and returns a random avatar image unique to input data
// in custom size (height and width).
func RandomImageSize(size int, data []byte) (image.Image, error) {
	randExtent := len(palette.WebSafe) - 32
	rand.Seed(time.Now().UnixNano())
	colorIndex := rand.Intn(randExtent)
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

// RandomImage generates and returns a random avatar image unique to input data
// in default size (height and width).
func RandomImage(data []byte) (image.Image, error) {
	return RandomImageSize(AvatarSize, data)
}
