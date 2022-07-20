// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package robot

import (
	"fmt"
	"image"
	"image/draw"

	"github.com/lafriks/go-avatars"
)

// Robot is used to generate pseudo-random avatars
type Robot struct{}

func (_ Robot) Name() string {
	return "robot"
}

func (_ Robot) RandomUserImage(size int, data []byte) (image.Image, error) {
	a, err := avatars.Generate(string(data))
	if err != nil {
		return nil, err
	}
	return a.Image(avatars.RenderSize(size))
}

func (_ Robot) RandomOrgImage(size int, data []byte) (image.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, size*2, size*2))

	for i := 0; i < 4; i++ {
		a, err := avatars.Generate(fmt.Sprintf("%s-%d", string(data), i))
		if err != nil {
			return nil, err
		}
		av, err := a.Image(avatars.RenderSize(size))
		if err != nil {
			return nil, err
		}
		pos := image.Rect((i-int(i/2)*2)*size, int(i/2)*size, ((i-int(i/2)*2)+1)*size, (int(i/2)+1)*size)
		draw.Draw(img, pos, av, image.Point{}, draw.Over)
	}

	return img, nil
}
