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

func Test_PrepareWithPNG(t *testing.T) {
	setting.Avatar.MaxWidth = 4096
	setting.Avatar.MaxHeight = 4096

	data, err := os.ReadFile("testdata/avatar.png")
	assert.NoError(t, err)

	img, err := resizeAvatar(data)
	assert.NoError(t, err)

	assert.Equal(t, DefaultAvatarSize, img.Bounds().Max.X)
	assert.Equal(t, DefaultAvatarSize, img.Bounds().Max.Y)
}

func Test_PrepareWithJPEG(t *testing.T) {
	setting.Avatar.MaxWidth = 4096
	setting.Avatar.MaxHeight = 4096

	data, err := os.ReadFile("testdata/avatar.jpeg")
	assert.NoError(t, err)

	img, err := resizeAvatar(data)
	assert.NoError(t, err)

	assert.Equal(t, DefaultAvatarSize, img.Bounds().Max.X)
	assert.Equal(t, DefaultAvatarSize, img.Bounds().Max.Y)
}

func Test_PrepareWithInvalidImage(t *testing.T) {
	setting.Avatar.MaxWidth = 5
	setting.Avatar.MaxHeight = 5

	_, err := resizeAvatar([]byte{})
	assert.EqualError(t, err, "image.DecodeConfig: image: unknown format")
}

func Test_PrepareWithInvalidImageSize(t *testing.T) {
	setting.Avatar.MaxWidth = 5
	setting.Avatar.MaxHeight = 5

	data, err := os.ReadFile("testdata/avatar.png")
	assert.NoError(t, err)

	_, err = resizeAvatar(data)
	assert.EqualError(t, err, "image width is too large: 10 > 5")
}

func Test_TryToResizeAvatar(t *testing.T) {
	setting.Avatar.MaxWidth = 4096
	setting.Avatar.MaxHeight = 4096

	newImgData := func(size int) []byte {
		img := image.NewRGBA(image.Rect(0, 0, size, size))
		bs := bytes.Buffer{}
		err := png.Encode(&bs, img)
		assert.NoError(t, err)
		return bs.Bytes()
	}

	// if origin image is smaller than the default size, use the origin image
	origin := newImgData(1)
	resized, err := TryToResizeAvatar(origin, 0)
	assert.NoError(t, err)
	assert.Equal(t, origin, resized)

	// use the resized image if the resized is smaller
	origin = newImgData(DefaultAvatarSize + 100)
	resized, err = TryToResizeAvatar(origin, 0)
	assert.NoError(t, err)
	assert.Less(t, len(resized), len(origin))

	// still use the origin image if the origin doesn't exceed the max-origin-size
	origin = newImgData(DefaultAvatarSize + 100)
	resized, err = TryToResizeAvatar(origin, 128000)
	assert.NoError(t, err)
	assert.Equal(t, origin, resized)

	// allow to use known image format (eg: webp) if it is small enough
	origin, err = os.ReadFile("testdata/animated.webp")
	assert.NoError(t, err)
	resized, err = TryToResizeAvatar(origin, 128000)
	assert.NoError(t, err)
	assert.Equal(t, origin, resized)

	// if a format is known, but it's not convertable, then it can't be used
	origin, err = os.ReadFile("testdata/animated.webp")
	width, height, acceptable := detectAcceptableWebp(origin)
	assert.EqualValues(t, 400, width)
	assert.EqualValues(t, 400, height)
	assert.True(t, acceptable)
	assert.NoError(t, err)
	_, err = TryToResizeAvatar(origin, 0)
	assert.ErrorContains(t, err, "image data size is too large and it can't be converted")

	// do not support unknown image formats, eg: SVG may contain embedded JS
	origin = []byte("<svg></svg>")
	_, err = TryToResizeAvatar(origin, 128000)
	assert.ErrorContains(t, err, "unsupported image format")
}
