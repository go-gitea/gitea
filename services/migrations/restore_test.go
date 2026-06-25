// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRepositoryRestorer_GetReleases_LocalFileInclusion ensures a crafted
// release.yml cannot make the restorer read files outside of the dump
// directory via a path-traversal DownloadURL.
func TestRepositoryRestorer_GetReleases_LocalFileInclusion(t *testing.T) {
	baseDir := t.TempDir()

	// a legitimate attachment that lives inside the dump directory
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "good.txt"), []byte("ok"), 0o644))

	releaseYML := `- tag_name: v0.0.1
  name: test
  assets:
    - name: good.txt
      download_url: good.txt
      size: 2
      download_count: 0
    - name: evil.txt
      download_url: ../../../../../../../../etc/passwd
      size: 0
      download_count: 0
`
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "release.yml"), []byte(releaseYML), 0o644))

	r, err := NewRepositoryRestorer(context.Background(), baseDir, "owner", "repo", false)
	require.NoError(t, err)

	releases, err := r.GetReleases(context.Background())
	require.NoError(t, err)
	require.Len(t, releases, 1)
	require.Len(t, releases[0].Assets, 2)

	// the in-dump asset keeps a file:// URL pointing inside baseDir
	good := releases[0].Assets[0]
	require.NotNil(t, good.DownloadURL)
	assert.Equal(t, "file://"+filepath.Join(baseDir, "good.txt"), *good.DownloadURL)

	// the traversal asset must be neutralised: DownloadURL is cleared so no
	// file outside baseDir is ever opened.
	evil := releases[0].Assets[1]
	assert.Nil(t, evil.DownloadURL, "path-traversal DownloadURL must be rejected")
}

// TestRepositoryRestorer_localFileURL covers the containment helper directly.
func TestRepositoryRestorer_localFileURL(t *testing.T) {
	baseDir := t.TempDir()
	r := &RepositoryRestorer{baseDir: baseDir}

	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"plain file", "attachment.bin", false},
		{"nested file", "assets/attachment.bin", false},
		{"parent traversal", "../escape", true},
		{"deep traversal", "../../../../etc/passwd", true},
		{"absolute path escapes via traversal", "/../../etc/passwd", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := r.localFileURL(c.in)
			if c.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.True(t, got == "file://"+baseDir ||
				len(got) > len("file://"+baseDir) && got[:len("file://"+baseDir+string(os.PathSeparator))] == "file://"+baseDir+string(os.PathSeparator),
				"resolved URL %q must stay within baseDir", got)
		})
	}
}
