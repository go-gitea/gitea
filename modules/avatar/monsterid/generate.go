// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package monsterid

import (
	"image"

	monster "src.techknowlogick.com/monster-id"
)

// Monster is used to generate pseudo-random avatars
type Monster struct{}

func (Monster) Name() string {
	return "monsterid"
}

func (Monster) RandomUserImage(size int, data []byte) (image.Image, error) {
	return randomImageSize(size, data)
}

func (Monster) RandomOrgImage(size int, data []byte) (image.Image, error) {
	return randomImageSize(size, data)
}

func (Monster) RandomRepoImage(size int, data []byte) (image.Image, error) {
	return randomImageSize(size, data)
}

func randomImageSize(size int, data []byte) (image.Image, error) {
	return monster.New(data), nil
}
