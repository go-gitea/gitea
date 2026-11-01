// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pages_model "gitea.dev/models/pages"
	"gitea.dev/models/unittest"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/util"
	pages_service "gitea.dev/services/pages"

	"github.com/stretchr/testify/require"
)

func TestPagesVersionMetadata(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte("version metadata"), 0o600))
	var archive bytes.Buffer
	require.NoError(t, pages_content.PackDirectory(t.Context(), root, &archive, pages_content.DefaultLimits()))
	created, err := pages_service.CommitUploadContent(t.Context(), 1, bytes.NewReader(archive.Bytes()))
	require.NoError(t, err)
	site, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	previousUpdate := site.PagesUpdatedUnixNano

	comment := "  production candidate  "
	keepForever := true
	updated, err := pages_service.UpdateVersionMetadata(t.Context(), 1, created.Version.ID, pages_service.VersionMetadataUpdate{
		Comment: &comment, KeepForever: &keepForever,
	})
	require.NoError(t, err)
	require.Equal(t, "production candidate", updated.Comment)
	require.True(t, updated.KeepForever)
	site, _, err = pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.Equal(t, previousUpdate, site.PagesUpdatedUnixNano)

	reused, err := pages_service.CommitUploadContent(t.Context(), 1, bytes.NewReader(archive.Bytes()))
	require.NoError(t, err)
	require.Equal(t, created.Version.ID, reused.Version.ID)
	require.Equal(t, updated.Comment, reused.Version.Comment)
	require.True(t, reused.Version.KeepForever)
	require.Equal(t, previousUpdate, reused.PagesUpdatedUnixNano)

	newline := "first\nsecond"
	_, err = pages_service.UpdateVersionMetadata(t.Context(), 1, created.Version.ID, pages_service.VersionMetadataUpdate{Comment: &newline})
	require.ErrorIs(t, err, util.ErrInvalidArgument)
	tooLong := strings.Repeat("a", pages_model.MaxVersionCommentLength+1)
	_, err = pages_service.UpdateVersionMetadata(t.Context(), 1, created.Version.ID, pages_service.VersionMetadataUpdate{Comment: &tooLong})
	require.ErrorIs(t, err, util.ErrInvalidArgument)
	unicodeComment := strings.Repeat("页", pages_model.MaxVersionCommentLength)
	updated, err = pages_service.UpdateVersionMetadata(t.Context(), 1, created.Version.ID, pages_service.VersionMetadataUpdate{Comment: &unicodeComment})
	require.NoError(t, err)
	require.Equal(t, unicodeComment, updated.Comment)
	_, err = pages_service.UpdateVersionMetadata(t.Context(), 1, created.Version.ID, pages_service.VersionMetadataUpdate{})
	require.ErrorIs(t, err, util.ErrInvalidArgument)
	_, err = pages_service.UpdateVersionMetadata(t.Context(), 2, created.Version.ID, pages_service.VersionMetadataUpdate{KeepForever: &keepForever})
	require.ErrorIs(t, err, util.ErrNotExist)
}
