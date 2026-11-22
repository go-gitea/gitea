// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"
	"time"

	"gitea.dev/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_GetCodeActivityStats(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepositoryLocal(t.Context(), bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	timeFrom, err := time.Parse(time.RFC3339, "2016-01-01T00:00:00+00:00")
	assert.NoError(t, err)

	code, err := bareRepo1.GetCodeActivityStats(t.Context(), timeFrom, "")
	assert.NoError(t, err)
	assert.NotNil(t, code)

	assert.EqualValues(t, 10, code.CommitCount)
	assert.EqualValues(t, 3, code.AuthorCount)
	assert.EqualValues(t, 10, code.CommitCountInAllBranches)
	assert.EqualValues(t, 10, code.Additions)
	assert.EqualValues(t, 1, code.Deletions)
	assert.Len(t, code.Authors, 3)
	assert.Equal(t, "tris.git@shoddynet.org", code.Authors[1].Email)
	assert.EqualValues(t, 3, code.Authors[1].Commits)
	assert.EqualValues(t, 5, code.Authors[0].Commits)

	authors, err := GetCodeActivityAuthors(t.Context(), bareRepo1, timeFrom)
	assert.NoError(t, err)
	assert.Equal(t, code.Authors, authors)
}

func TestRepository_GetCodeActivityStatsPathWithSpaces(t *testing.T) {
	repoDir := filepath.Join(t.TempDir(), "repo.git")
	require.NoError(t, gitcmd.NewCommand("init", "--bare").AddDynamicArguments(repoDir).RunWithStderr(t.Context()))
	stdin := `blob
mark :1
data 2
a

commit refs/heads/master
author test <test@example.com> 1769202331 +0000
committer test <test@example.com> 1769202331 +0000
data 2
A
M 100644 :1 docs/release notes.md
M 100644 :1 docs/release plan.md
`
	require.NoError(t, gitcmd.NewCommand("fast-import").WithDir(repoDir).WithStdinBytes([]byte(stdin)).RunWithStderr(t.Context()))
	repo, err := OpenRepositoryLocal(t.Context(), repoDir)
	require.NoError(t, err)
	defer repo.Close()

	code, err := repo.GetCodeActivityStats(t.Context(), time.Unix(1769202000, 0), "")
	require.NoError(t, err)
	assert.EqualValues(t, 2, code.Additions)
	assert.EqualValues(t, 2, code.ChangedFiles)
}
