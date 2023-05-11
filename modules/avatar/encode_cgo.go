// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build cgo

package avatar

import (
	"image"
	"io"

	"code.gitea.io/gitea/modules/log"

	"github.com/chai2010/webp"
)

// Encoder returns a function that can be used to encode an avatar image as webp
func Encoder(img image.Image) func(io.Writer) error {
	return func(w io.Writer) error {
		if err := webp.Encode(w, img, &webp.Options{Quality: 75}); err != nil {
			log.Error("Unable to Encode image to webp: %v", err)
		}
		return nil
	}
}
