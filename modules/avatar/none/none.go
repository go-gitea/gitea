// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package none

import (
	"image"
)

// None wont generate an image
type None struct{}

func (None) Name() string {
	return "none"
}

func (None) RandomUserImage(size int, data []byte) (image.Image, error) {
	return nil, nil
}

func (None) RandomOrgImage(size int, data []byte) (image.Image, error) {
	return nil, nil
}

func (None) RandomRepoImage(size int, data []byte) (image.Image, error) {
	return nil, nil
}
