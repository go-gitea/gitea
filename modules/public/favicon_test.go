// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"os"
	"path/filepath"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFaviconVariantAvailable(t *testing.T) {
	defer test.MockVariableValue(&setting.CustomPath, t.TempDir())()

	assert.True(t, FaviconVariantAvailable(FaviconVariantSuccess))
	assert.False(t, FaviconVariantAvailable("unknown"))
}

func TestFaviconVariantAvailableWithCustomFavicon(t *testing.T) {
	customPath := t.TempDir()
	defer test.MockVariableValue(&setting.CustomPath, customPath)()

	imgPath := filepath.Join(customPath, "public", "assets", "img")
	require.NoError(t, os.MkdirAll(imgPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(imgPath, "favicon.svg"), []byte("<svg/>"), 0o644))

	assert.False(t, FaviconVariantAvailable(FaviconVariantSuccess))

	for _, variant := range faviconVariants {
		require.NoError(t, os.WriteFile(filepath.Join(imgPath, "favicon-"+variant+".svg"), []byte("<svg/>"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(imgPath, "favicon-"+variant+".png"), []byte("png"), 0o644))
	}

	assert.True(t, FaviconVariantAvailable(FaviconVariantSuccess))
}
