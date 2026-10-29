// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	pages_model "gitea.dev/models/pages"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/log"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"

	"github.com/stretchr/testify/require"
)

func TestPagesDoctorProtectsRecentOrReferencedObjects(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	root := t.TempDir()
	backend, err := storage.NewLocalStorage(t.Context(), &setting.Storage{Path: root})
	require.NoError(t, err)
	previous := storage.Pages
	storage.Pages = backend
	t.Cleanup(func() { storage.Pages = previous })

	currentDigest := bytes.Repeat([]byte("a"), 64)
	require.NoError(t, pages_model.UpsertSite(t.Context(), &pages_model.Site{
		RepoID: 1, SourceType: "upload", TarGzSHA256: string(currentDigest), PagesUpdatedUnixNano: time.Now().UnixNano(),
	}))
	currentPath, err := pages_content.ContentRelativePath(1, string(currentDigest))
	require.NoError(t, err)
	oldPath, err := pages_content.ContentRelativePath(9998, string(bytes.Repeat([]byte("b"), 64)))
	require.NoError(t, err)
	recentPath, err := pages_content.ContentRelativePath(9999, string(bytes.Repeat([]byte("c"), 64)))
	require.NoError(t, err)
	for _, objectPath := range []string{currentPath, oldPath, recentPath} {
		_, err := backend.Save(objectPath, bytes.NewReader([]byte("data")), 4)
		require.NoError(t, err)
	}
	oldTime := time.Now().Add(-25 * time.Hour)
	require.NoError(t, os.Chtimes(filepath.Join(root, filepath.FromSlash(oldPath)), oldTime, oldTime))

	logger := log.GetLogger("doctor")
	require.NoError(t, checkPagesStorage(t.Context(), logger, false))
	_, err = backend.Stat(oldPath)
	require.NoError(t, err, "online inspection only reports orphans")
	require.NoError(t, checkPagesStorage(t.Context(), logger, true))
	_, err = backend.Stat(oldPath)
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = backend.Stat(recentPath)
	require.NoError(t, err, "objects younger than 24 hours remain")
	_, err = backend.Stat(currentPath)
	require.NoError(t, err, "the current digest remains regardless of age")
}
