// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func Test_RandomImageSize(t *testing.T) {
	_, err := RandomImageSize(0, []byte("gitea@local"))
	assert.Error(t, err)

	_, err = RandomImageSize(64, []byte("gitea@local"))
	assert.NoError(t, err)
}

func Test_RandomImage(t *testing.T) {
	_, err := RandomImage([]byte("gitea@local"))
	assert.NoError(t, err)
}

func Test_ProcessAvatarPNG(t *testing.T) {
	setting.Avatar.MaxWidth = 4096
	setting.Avatar.MaxHeight = 4096

	data, err := os.ReadFile("testdata/avatar.png")
	assert.NoError(t, err)

	_, err = processAvatarImage(data, 262144)
	assert.NoError(t, err)
}

func Test_ProcessAvatarJPEG(t *testing.T) {
	setting.Avatar.MaxWidth = 4096
	setting.Avatar.MaxHeight = 4096

	data, err := os.ReadFile("testdata/avatar.jpeg")
	assert.NoError(t, err)

	_, err = processAvatarImage(data, 262144)
	assert.NoError(t, err)
}

func Test_ProcessAvatarInvalidData(t *testing.T) {
	setting.Avatar.MaxWidth = 5
	setting.Avatar.MaxHeight = 5

	_, err := processAvatarImage([]byte{}, 12800)
	assert.EqualError(t, err, "image.DecodeConfig: image: unknown format")
}

func Test_ProcessAvatarInvalidImageSize(t *testing.T) {
	setting.Avatar.MaxWidth = 5
	setting.Avatar.MaxHeight = 5

	data, err := os.ReadFile("testdata/avatar.png")
	assert.NoError(t, err)

	_, err = processAvatarImage(data, 12800)
	assert.EqualError(t, err, "image width is too large: 10 > 5")
}

func Test_ProcessAvatarImage(t *testing.T) {
	setting.Avatar.MaxWidth = 4096
	setting.Avatar.MaxHeight = 4096
	scaledSize := DefaultAvatarSize * setting.Avatar.RenderedSizeFactor

	newImgData := func(size int, optHeight ...int) []byte {
		width := size
		height := size
		if len(optHeight) == 1 {
			height = optHeight[0]
		}
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		bs := bytes.Buffer{}
		err := png.Encode(&bs, img)
		assert.NoError(t, err)
		return bs.Bytes()
	}

	// if origin image canvas is too large, crop and resize it
	origin := newImgData(500, 600)
	result, err := processAvatarImage(origin, 0)
	assert.NoError(t, err)
	assert.NotEqual(t, origin, result)
	decoded, err := png.Decode(bytes.NewReader(result))
	assert.NoError(t, err)
	assert.EqualValues(t, scaledSize, decoded.Bounds().Max.X)
	assert.EqualValues(t, scaledSize, decoded.Bounds().Max.Y)

	// if origin image is smaller than the default size, use the origin image
	origin = newImgData(1)
	result, err = processAvatarImage(origin, 0)
	assert.NoError(t, err)
	assert.Equal(t, origin, result)

	// use the origin image if the origin is smaller
	origin = newImgData(scaledSize + 100)
	result, err = processAvatarImage(origin, 0)
	assert.NoError(t, err)
	assert.Less(t, len(result), len(origin))

	// still use the origin image if the origin doesn't exceed the max-origin-size
	origin = newImgData(scaledSize + 100)
	result, err = processAvatarImage(origin, 262144)
	assert.NoError(t, err)
	assert.Equal(t, origin, result)

	// allow to use known image format (eg: webp) if it is small enough
	origin, err = os.ReadFile("testdata/animated.webp")
	assert.NoError(t, err)
	result, err = processAvatarImage(origin, 262144)
	assert.NoError(t, err)
	assert.Equal(t, origin, result)

	// do not support unknown image formats, eg: SVG may contain embedded JS
	origin = []byte("<svg></svg>")
	_, err = processAvatarImage(origin, 262144)
	assert.ErrorContains(t, err, "image: unknown format")

	// make sure the canvas size limit works
	setting.Avatar.MaxWidth = 5
	setting.Avatar.MaxHeight = 5
	origin = newImgData(10)
	_, err = processAvatarImage(origin, 262144)
	assert.ErrorContains(t, err, "image width is too large: 10 > 5")
}
