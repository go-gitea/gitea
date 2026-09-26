// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"os"
	"path/filepath"
	"testing"

	"gitea.dev/modules/optional"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRepositoryRestorer_GetReleases_LocalFileInclusion ensures a crafted
// release.yml cannot make the restorer read files outside the dump
// directory via a path-traversal DownloadURL.
func TestRepositoryRestorer_GetReleases_LocalFileInclusion(t *testing.T) {
	baseDir := t.TempDir()

	// a legitimate attachment that lives inside the dump directory
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "good.txt"), []byte("ok"), 0o644))

	releaseYML := `
- assets:
    - name: good.txt
      download_url: good.txt
    - name: evil.txt
      download_url: ../../../../../../../../etc/passwd
`
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "release.yml"), []byte(releaseYML), 0o644))

	r, err := NewRepositoryRestorer(t.Context(), baseDir, "owner", "repo", false)
	require.NoError(t, err)

	releases, err := r.GetReleases(t.Context())
	require.NoError(t, err)
	require.Len(t, releases, 1)
	require.Len(t, releases[0].Assets, 2)

	// the in-dump asset keeps a file:// URL pointing inside baseDir
	assets := releases[0].Assets
	assert.Equal(t, "file://"+filepath.Join(baseDir, "good.txt"), optional.FromPtr(assets[0].DownloadURL).Value())
	assert.Equal(t, "file://"+filepath.Join(baseDir, "etc/passwd"), optional.FromPtr(assets[1].DownloadURL).Value())
}

func TestRepositoryRestorer_GetPullRequestsStripsUnsafeCloneURL(t *testing.T) {
	baseDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "change.patch"), []byte("patch"), 0o644))

	pullRequestYML := `
- number: 1
  patch_url: change.patch
  head:
    clone_url: http://127.0.0.1/private.git
    ref: feature
  base:
    ref: main
`
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "pull_request.yml"), []byte(pullRequestYML), 0o644))

	r, err := NewRepositoryRestorer(t.Context(), baseDir, "owner", "repo", false)
	require.NoError(t, err)

	pulls, _, err := r.GetPullRequests(t.Context(), 1, 10)
	require.NoError(t, err)
	require.Len(t, pulls, 1)

	assert.Equal(t, "file://"+filepath.Join(baseDir, "change.patch"), pulls[0].PatchURL)
	assert.Empty(t, pulls[0].Head.CloneURL)
	assert.True(t, pulls[0].EnsuredSafe)
}
