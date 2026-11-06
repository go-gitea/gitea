// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrepSearch(t *testing.T) {
	defer test.MockVariableValue(&setting.RepoRootPath, t.TempDir())()
	repo, err := OpenRepositoryLocal(t.Context(), filepath.Join(testReposDir, "language_stats_repo"))
	require.NoError(t, err)
	defer repo.Close()
	for _, testCase := range []struct {
		name       string
		patterns   []string
		paths      []string
		limit      int
		expected   []string
		incomplete bool
	}{
		{name: "paths in order", patterns: []string{"(*UTF)void"}, expected: []string{"java-hello/main.java", "main.vendor.java"}},
		{name: "directory", patterns: []string{"(*UTF)void"}, paths: []string{":(literal)java-hello/"}, expected: []string{"java-hello/main.java"}},
		{name: "literal directory", patterns: []string{"(*UTF)void"}, paths: []string{":(literal)java-*/"}},
		{name: "cap", patterns: []string{"(*UTF)void"}, limit: 1, expected: []string{"java-hello/main.java"}, incomplete: true},
		{name: "exact cap", patterns: []string{"(*UTF)void"}, limit: 2, expected: []string{"java-hello/main.java", "main.vendor.java"}},
		{name: "no match", patterns: []string{"(*UTF)no-such-content"}},
		{name: "dash", patterns: []string{"(*UTF)-v"}, expected: []string{".gitattributes"}},
		{name: "no patterns"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var paths []string
			incomplete, err := GrepSearch(t.Context(), repo, testCase.patterns, "HEAD", func(filename string) (bool, error) {
				if testCase.limit > 0 && len(paths) == testCase.limit {
					return false, nil
				}
				paths = append(paths, filename)
				return true, nil
			}, testCase.paths...)
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, paths)
			assert.Equal(t, testCase.incomplete, incomplete)
		})
	}
	visit := func(_filename string) (bool, error) { return true, nil }
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = GrepSearch(ctx, repo, []string{"(*UTF)void"}, "HEAD", visit)
	assert.ErrorIs(t, err, context.Canceled)
	_, err = GrepSearch(t.Context(), repo, []string{"(*UTF)void"}, "HEAD", func(_filename string) (bool, error) {
		return false, errors.New("read failed")
	})
	assert.ErrorContains(t, err, "read failed")
	nonExistingRepo := &Repository{RepositoryBase: RepositoryBase{repoFacade: gitrepo.RepositoryUnmanaged("no-such-git-repo")}}
	_, err = GrepSearch(t.Context(), nonExistingRepo, []string{"(*UTF)void"}, "HEAD", visit)
	assert.Error(t, err)
}
