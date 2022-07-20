// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dicebear

import (
	"fmt"
	"image"
	"image/draw"
	"strings"

	"codeberg.org/Codeberg/avatars"
	"github.com/fogleman/gg"
	"github.com/lafriks/go-svg"
	"github.com/lafriks/go-svg/renderer"
	rendr_gg "github.com/lafriks/go-svg/renderer/gg"
)

// DiceBear is used to generate pseudo-random avatars
type DiceBear struct{}

func (DiceBear) Name() string {
	return "dicebear"
}

func (DiceBear) RandomUserImage(size int, data []byte) (image.Image, error) {
	return randomImageSize(size, data)
}

func (DiceBear) RandomOrgImage(size int, data []byte) (image.Image, error) {
	size /= 2
	space := size / 20
	img := image.NewRGBA(image.Rect(0, 0, size*2, size*2))

	for i := 0; i < 4; i++ {
		av, err := randomImageSize(size, []byte(fmt.Sprintf("%s-%d", string(data), i)))
		if err != nil {
			return nil, err
		}
		pos := image.Rect((i-(i/2)*2)*(size+space), (i/2)*(size+space), ((i-(i/2)*2)+1)*(size+space), ((i/2)+1)*(size+space))
		draw.Draw(img, pos, av, image.Point{}, draw.Over)
	}

	return img, nil
}

func randomImageSize(size int, data []byte) (image.Image, error) {
	svgAvatar := avatars.MakeAvatar(string(data))

	s, err := svg.Parse(strings.NewReader(svgAvatar), svg.IgnoreErrorMode)
	if err != nil {
		return nil, err
	}

	gc := gg.NewContext(size, size)
	rendr_gg.Draw(gc, s, renderer.Target(0, 0, float64(size), float64(size)))

	return gc.Image(), nil
}
