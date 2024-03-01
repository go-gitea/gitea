// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestLookupRedirect(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repoID, err := repo_model.LookupRedirect(db.DefaultContext, 2, "oldrepo1")
	assert.NoError(t, err)
	assert.EqualValues(t, 1, repoID)

	_, err = repo_model.LookupRedirect(db.DefaultContext, unittest.NonexistentID, "doesnotexist")
	assert.True(t, repo_model.IsErrRedirectNotExist(err))
}

func TestNewRedirect(t *testing.T) {
	// redirect to a completely new name
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.NoError(t, repo_model.NewRedirect(db.DefaultContext, repo.OwnerID, repo.ID, repo.Name, "newreponame"))

	unittest.AssertExistsAndLoadBean(t, &repo_model.Redirect{
		OwnerID:        repo.OwnerID,
		LowerName:      repo.LowerName,
		RedirectRepoID: repo.ID,
	})
	unittest.AssertExistsAndLoadBean(t, &repo_model.Redirect{
		OwnerID:        repo.OwnerID,
		LowerName:      "oldrepo1",
		RedirectRepoID: repo.ID,
	})
}

func TestNewRedirect2(t *testing.T) {
	// redirect to previously used name
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.NoError(t, repo_model.NewRedirect(db.DefaultContext, repo.OwnerID, repo.ID, repo.Name, "oldrepo1"))

	unittest.AssertExistsAndLoadBean(t, &repo_model.Redirect{
		OwnerID:        repo.OwnerID,
		LowerName:      repo.LowerName,
		RedirectRepoID: repo.ID,
	})
	unittest.AssertNotExistsBean(t, &repo_model.Redirect{
		OwnerID:        repo.OwnerID,
		LowerName:      "oldrepo1",
		RedirectRepoID: repo.ID,
	})
}

func TestNewRedirect3(t *testing.T) {
	// redirect for a previously-unredirected repo
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	assert.NoError(t, repo_model.NewRedirect(db.DefaultContext, repo.OwnerID, repo.ID, repo.Name, "newreponame"))

	unittest.AssertExistsAndLoadBean(t, &repo_model.Redirect{
		OwnerID:        repo.OwnerID,
		LowerName:      repo.LowerName,
		RedirectRepoID: repo.ID,
	})
}
