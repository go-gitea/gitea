// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wavatars

import (
	"image"

	"src.techknowlogick.com/wavatars"
)

// Wavatars is used to generate pseudo-random avatars
type Wavatars struct{}

func (Wavatars) Name() string {
	return "wavatars"
}

func (Wavatars) RandomUserImage(size int, data []byte) (image.Image, error) {
	return randomImageSize(size, data)
}

func (Wavatars) RandomOrgImage(size int, data []byte) (image.Image, error) {
	return randomImageSize(size, data)
}

func (Wavatars) RandomRepoImage(size int, data []byte) (image.Image, error) {
	return randomImageSize(size, data)
}

func randomImageSize(size int, data []byte) (image.Image, error) {
	return wavatars.New(data), nil
}
