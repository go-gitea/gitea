// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPushUpdateAddTagsCaseSensitive(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	lower := unittest.AssertExistsAndLoadBean(t, &repo_model.Release{ID: 1})
	gitRepo, err := git.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(t.Context(), "branch2")
	require.NoError(t, err)
	_, _, err = gitcmd.NewCommand("pack-refs", "--all").WithRepo(repo).RunStdString(t.Context())
	require.NoError(t, err)
	require.NoError(t, gitRepo.CreateTag(t.Context(), "V1.1", commit.ID.String()))
	_, _, err = gitcmd.NewCommand("pack-refs", "--all").WithRepo(repo).RunStdString(t.Context())
	require.NoError(t, err)
	require.NoError(t, PushUpdateAddDeleteTags(t.Context(), repo, gitRepo, &user_model.User{ID: repo.OwnerID}, []string{"v1.1", "V1.1"}, nil))

	upper, err := repo_model.GetRelease(t.Context(), repo.ID, "V1.1")
	require.NoError(t, err)
	assert.Equal(t, commit.ID.String(), upper.Sha1)
	assert.True(t, upper.IsTag)
	assert.Equal(t, lower.Sha1, unittest.AssertExistsAndLoadBean(t, &repo_model.Release{ID: lower.ID}).Sha1)
}
