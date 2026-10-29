// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPagesStorageInheritsGlobalStorage(t *testing.T) {
	cfg, err := NewConfigProviderFromData("[storage]\nSTORAGE_TYPE = minio\n")
	assert.NoError(t, err)
	assert.NoError(t, loadPagesFrom(cfg))
	assert.Equal(t, MinioStorageType, Pages.Storage.Type)
	assert.Equal(t, "pages/", Pages.Storage.MinioConfig.BasePath)
}

func TestPagesStorageCanOverridePath(t *testing.T) {
	cfg, err := NewConfigProviderFromData("[storage.pages]\nPATH = page-content\n")
	assert.NoError(t, err)
	assert.NoError(t, loadPagesFrom(cfg))
	assert.Equal(t, LocalStorageType, Pages.Storage.Type)
	assert.Equal(t, filepath.Join(AppDataPath, "page-content"), Pages.Storage.Path)
}
