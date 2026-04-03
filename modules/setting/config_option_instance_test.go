// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebBannerTypeColors(t *testing.T) {
	t.Run("normalized", func(t *testing.T) {
		banner := WebBannerType{BackgroundColor: "abc"}

		assert.Equal(t, "#aabbcc", banner.NormalizedBackgroundColor())
		assert.Equal(t, "#000", banner.TextColor())
		assert.Equal(t, "#8b99a7", banner.BorderColor())
	})

	t.Run("default form color", func(t *testing.T) {
		banner := WebBannerType{}

		assert.Equal(t, defaultWebBannerBackgroundColor, banner.BackgroundColorForForm())
		assert.Equal(t, defaultWebBannerBackgroundColor, banner.FormValue().BackgroundColor)
	})

	t.Run("invalid configured color", func(t *testing.T) {
		banner := WebBannerType{BackgroundColor: "invalid"}

		assert.Empty(t, banner.NormalizedBackgroundColor())
		assert.Empty(t, banner.TextColor())
		assert.Empty(t, banner.BorderColor())
		assert.Equal(t, defaultWebBannerBackgroundColor, banner.BackgroundColorForForm())
	})
}
