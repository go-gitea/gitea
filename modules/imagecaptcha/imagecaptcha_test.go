// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package imagecaptcha

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"strings"
	"testing"

	"gitea.dev/modules/cache"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageCaptcha(t *testing.T) {
	require.NoError(t, cache.Init())
	createWithAnswer := func() (string, string) {
		id, err := Create()
		require.NoError(t, err)
		digits, ok := cache.GetCache().Get(cacheKeyPrefix + id)
		require.True(t, ok)
		return id, strings.Map(func(digit rune) rune { return digit + '0' }, digits)
	}

	id, answer := createWithAnswer()
	assert.Len(t, answer, digitCount)
	assert.True(t, Verify(id, answer))
	assert.False(t, Verify(id, answer))

	id, answer = createWithAnswer()
	assert.False(t, Verify(id, "wrong"))
	assert.False(t, Verify(id, answer))
	assert.False(t, Verify("", ""))
	assert.False(t, Verify("unknown", answer))

	_, err := renderImage("unknown", true)
	assert.ErrorIs(t, err, util.ErrNotExist)
	_, exists := cache.GetCache().Get(cacheKeyPrefix + "unknown")
	assert.False(t, exists)

	id, _ = createWithAnswer()
	first, err := renderImage(id, false)
	require.NoError(t, err)
	decoded, err := png.Decode(bytes.NewReader(first))
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, imageWidth, imageHeight), decoded.Bounds())
	second, err := renderImage(id, false)
	require.NoError(t, err)
	assert.Equal(t, first, second)

	id, _ = createWithAnswer()
	require.NoError(t, cache.GetCache().Delete(cacheKeyPrefix+id))
	_, err = renderImage(id, true)
	require.NoError(t, err)
	_, exists = cache.GetCache().Get(cacheKeyPrefix + id)
	assert.True(t, exists)
}

func TestPaletteUsesEveryColorAndSaturatedChannels(t *testing.T) {
	seen := map[color.Color]bool{}
	for seed := range 100 {
		seen[newPalette(rand.New(rand.NewChaCha8([32]byte{byte(seed)})))[1]] = true
	}
	assert.Len(t, seen, len(primaryColors))
	assert.NotPanics(t, func() { randomBrightness(rand.New(rand.NewPCG(1, 2)), color.RGBA{R: 255, A: 255}) })
}
