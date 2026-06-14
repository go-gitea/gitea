// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gitea.dev/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchRemoteCommitConcurrentSameCommit(t *testing.T) {
	sourceRepo, commitIDs := createFetchRemoteCommitSourceRepo(t, 8)
	targetRepo := createBareRepo(t, "target.git")
	commitID := commitIDs[len(commitIDs)-1]

	const fetchers = 8
	errCh := make(chan error, fetchers)
	var wg sync.WaitGroup
	for range fetchers {
		wg.Go(func() {
			errCh <- FetchRemoteCommit(t.Context(), targetRepo, sourceRepo, commitID)
		})
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}

	assertFetchedCommitPresent(t, targetRepo, commitID)
	assertNoFetchHead(t, targetRepo)
	assertNoFetchedRefs(t, targetRepo)
}

func TestFetchRemoteCommitConcurrentDifferentCommits(t *testing.T) {
	sourceRepo, commitIDs := createFetchRemoteCommitSourceRepo(t, 8)
	targetRepo := createBareRepo(t, "target.git")

	errCh := make(chan error, len(commitIDs))
	var wg sync.WaitGroup
	for _, commitID := range commitIDs {
		wg.Add(1)
		go func(commitID string) {
			defer wg.Done()
			errCh <- FetchRemoteCommit(t.Context(), targetRepo, sourceRepo, commitID)
		}(commitID)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}

	for _, commitID := range commitIDs {
		assertFetchedCommitPresent(t, targetRepo, commitID)
	}
	assertNoFetchHead(t, targetRepo)
	assertNoFetchedRefs(t, targetRepo)
}

func createFetchRemoteCommitSourceRepo(t *testing.T, commitCount int) (*mockRepository, []string) {
	t.Helper()

	repoDir := filepath.Join(t.TempDir(), "source")
	require.NoError(t, gitcmd.NewCommand("init").AddDynamicArguments(repoDir).Run(t.Context()))
	require.NoError(t, gitcmd.NewCommand("config", "user.name", "User").WithDir(repoDir).Run(t.Context()))
	require.NoError(t, gitcmd.NewCommand("config", "user.email", "user@example.com").WithDir(repoDir).Run(t.Context()))

	for i := range commitCount {
		fileName := filepath.Join(repoDir, fmt.Sprintf("file-%d.txt", i))
		require.NoError(t, os.WriteFile(fileName, fmt.Appendf(nil, "content %d\n", i), 0o644))
		require.NoError(t, gitcmd.NewCommand("add", ".").WithDir(repoDir).Run(t.Context()))
		require.NoError(t, gitcmd.NewCommand("commit", "-m").AddDynamicArguments(fmt.Sprintf("commit %d", i)).WithDir(repoDir).Run(t.Context()))
	}

	stdout, _, runErr := gitcmd.NewCommand("rev-list", "--reverse", "HEAD").WithDir(repoDir).RunStdString(t.Context())
	require.NoError(t, runErr)

	commitIDs := strings.Fields(stdout)
	require.Len(t, commitIDs, commitCount)
	return &mockRepository{path: repoDir}, commitIDs
}

func createBareRepo(t *testing.T, name string) *mockRepository {
	t.Helper()

	repoDir := filepath.Join(t.TempDir(), name)
	require.NoError(t, gitcmd.NewCommand("init", "--bare").AddDynamicArguments(repoDir).Run(t.Context()))
	return &mockRepository{path: repoDir}
}

func assertFetchedCommitPresent(t *testing.T, repo *mockRepository, commitID string) {
	t.Helper()

	err := gitcmd.NewCommand("cat-file", "-e").WithDir(repo.path).AddDynamicArguments(commitID + "^{commit}").Run(t.Context())
	require.NoError(t, err)
}

func assertNoFetchHead(t *testing.T, repo *mockRepository) {
	t.Helper()

	_, err := os.Stat(filepath.Join(repo.path, "FETCH_HEAD"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func assertNoFetchedRefs(t *testing.T, repo *mockRepository) {
	t.Helper()

	stdout, _, runErr := gitcmd.NewCommand("for-each-ref", "--format=%(refname)").WithDir(repo.path).RunStdString(t.Context())
	require.NoError(t, runErr)
	assert.Empty(t, strings.TrimSpace(stdout))
}
