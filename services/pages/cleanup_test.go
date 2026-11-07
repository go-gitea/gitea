// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	"gitea.dev/models/unittest"
	pages_content "gitea.dev/modules/pages"
	pages_service "gitea.dev/services/pages"

	"github.com/stretchr/testify/require"
)

func TestCleanupPagesVersions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	store := pages_content.NewContentStore()
	versions := make([]*pages_model.Version, 0, 5)
	for index := range 5 {
		content := []byte{byte(index + 1)}
		digest := sha256.Sum256(content)
		version := &pages_model.Version{
			RepoID: 1, TarGzSHA256: hex.EncodeToString(digest[:]), OriginType: pages_model.VersionOriginUpload,
			KeepForever: index == 1,
		}
		require.NoError(t, pages_model.InsertVersion(t.Context(), version))
		require.NoError(t, store.Save(1, version.TarGzSHA256, bytes.NewReader(content), int64(len(content))))
		versions = append(versions, version)
	}
	require.NoError(t, pages_model.InsertSite(t.Context(), &pages_model.Site{
		RepoID: 1, Enabled: true, SourceType: pages_model.PublishSourceArtifact,
		CurrentVersionID: versions[0].ID, VersionRetentionCount: 2, PagesUpdatedUnixNano: 42,
	}))

	require.NoError(t, pages_service.CleanupVersions(t.Context()))
	retained, total, err := pages_model.FindVersions(t.Context(), &pages_model.FindVersionOptions{
		ListOptions: db.ListOptionsAll, RepoID: 1,
	})
	require.NoError(t, err)
	require.EqualValues(t, 4, total)
	retainedIDs := make(map[int64]struct{}, len(retained))
	for _, version := range retained {
		retainedIDs[version.ID] = struct{}{}
	}
	for _, version := range []*pages_model.Version{versions[0], versions[1], versions[3], versions[4]} {
		require.Contains(t, retainedIDs, version.ID)
		_, err := store.Stat(1, version.TarGzSHA256)
		require.NoError(t, err)
	}
	_, has, err := pages_model.GetVersionByID(t.Context(), versions[2].ID)
	require.NoError(t, err)
	require.False(t, has)
	_, err = store.Stat(1, versions[2].TarGzSHA256)
	require.ErrorIs(t, err, os.ErrNotExist)
	site, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, int64(42), site.PagesUpdatedUnixNano)

	secondRepoVersions := make([]*pages_model.Version, 0, 2)
	for _, content := range [][]byte{{10}, {11}} {
		digest := sha256.Sum256(content)
		version := &pages_model.Version{
			RepoID: 2, TarGzSHA256: hex.EncodeToString(digest[:]), OriginType: pages_model.VersionOriginUpload,
		}
		require.NoError(t, pages_model.InsertVersion(t.Context(), version))
		require.NoError(t, store.Save(2, version.TarGzSHA256, bytes.NewReader(content), int64(len(content))))
		secondRepoVersions = append(secondRepoVersions, version)
	}
	require.NoError(t, pages_model.InsertSite(t.Context(), &pages_model.Site{
		RepoID: 2, SourceType: pages_model.PublishSourceArtifact,
		CurrentVersionID: secondRepoVersions[1].ID, VersionRetentionCount: 1, PagesUpdatedUnixNano: 43,
	}))
	_, err = db.Exec(t.Context(), "UPDATE pages_site SET version_retention_count = 0 WHERE repo_id = ?", 1)
	require.NoError(t, err)
	require.NoError(t, pages_service.CleanupVersions(t.Context()))
	_, has, err = pages_model.GetVersionByID(t.Context(), secondRepoVersions[0].ID)
	require.NoError(t, err)
	require.False(t, has, "one invalid repository must not block cleanup for later repositories")
}
