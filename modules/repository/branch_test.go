// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	"gitea.dev/modules/gitrepo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncRepoBranches(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	_, err := db.GetEngine(t.Context()).ID(1).Update(&repo_model.Repository{ObjectFormatName: "bad-fmt"})
	assert.NoError(t, db.TruncateBeans(t.Context(), &git_model.Branch{}))
	assert.NoError(t, err)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Equal(t, "bad-fmt", repo.ObjectFormatName)
	_, err = SyncRepoBranches(t.Context(), 1, 0)
	assert.NoError(t, err)
	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Equal(t, "sha1", repo.ObjectFormatName)
	branch, err := git_model.GetBranch(t.Context(), 1, "master")
	assert.NoError(t, err)
	assert.Equal(t, "master", branch.Name)
}

func TestSyncRepoBranchesWithRepoSkipsAlreadyDeletedBranches(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	require.NoError(t, err)
	defer gitRepo.Close()

	require.NoError(t, git_model.AddBranches(ctx, []*git_model.Branch{{
		RepoID:    repo.ID,
		Name:      "already-deleted",
		CommitID:  git.Sha1ObjectFormat.EmptyObjectID().String(),
		IsDeleted: true,
	}}))

	_, results, err := SyncRepoBranchesWithRepo(ctx, repo, gitRepo, 0)
	require.NoError(t, err)

	for _, result := range results {
		assert.NotEqual(t, git.RefNameFromBranch("already-deleted"), result.RefName)
	}
}
