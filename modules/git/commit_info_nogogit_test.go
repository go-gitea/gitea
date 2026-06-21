// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
	"path/filepath"
	"testing"

	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntries_GetCommitsInfo_ContextErr(t *testing.T) {
	repo, err := OpenRepository(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
	require.NoError(t, err)
	defer repo.Close()

	commit, err := repo.GetCommit("feaf4ba6bc635fec442f46ddd4512416ec43c2c2")
	require.NoError(t, err)
	entries, err := commit.Tree.ListEntries()
	require.NoError(t, err)

	countCommitInfosCommit := func(infos []CommitInfo) (nilCommits, nonNilCommits int) {
		for _, info := range infos {
			nilCommits += util.Iif(info.Commit == nil, 1, 0)
			nonNilCommits += util.Iif(info.Commit != nil, 1, 0)
		}
		return nilCommits, nonNilCommits
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer test.MockVariableValue(&walkGitLogDebugBeforeNext)()

	walkGitLogDebugBeforeNext = cancel
	commitInfos, _, err := entries.GetCommitsInfo(ctx, "/any/repo-link", commit, "")
	assert.NoError(t, err)
	nilCommits, nonNilCommits := countCommitInfosCommit(commitInfos)
	assert.Equal(t, 0, nonNilCommits) // no commit info due to canceled (or deadline-exceeded) context
	assert.Equal(t, 3, nilCommits)

	walkGitLogDebugBeforeNext = nil
	commitInfos, _, err = entries.GetCommitsInfo(t.Context(), "/any/repo-link", commit, "")
	assert.NoError(t, err)
	nilCommits, nonNilCommits = countCommitInfosCommit(commitInfos)
	assert.Equal(t, 3, nonNilCommits)
	assert.Equal(t, 0, nilCommits)
}
