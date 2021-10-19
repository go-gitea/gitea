// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"github.com/stretchr/testify/assert"
)

func TestLookupRepoRedirect(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	repoID, err := LookupRepoRedirect(2, "oldrepo1")
	assert.NoError(t, err)
	assert.EqualValues(t, 1, repoID)

	_, err = LookupRepoRedirect(db.NonexistentID, "doesnotexist")
	assert.True(t, IsErrRepoRedirectNotExist(err))
}

func TestNewRepoRedirect(t *testing.T) {
	// redirect to a completely new name
	assert.NoError(t, db.PrepareTestDatabase())

	repo := db.AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.NoError(t, newRepoRedirect(db.GetEngine(db.DefaultContext), repo.OwnerID, repo.ID, repo.Name, "newreponame"))

	db.AssertExistsAndLoadBean(t, &RepoRedirect{
		OwnerID:        repo.OwnerID,
		LowerName:      repo.LowerName,
		RedirectRepoID: repo.ID,
	})
	db.AssertExistsAndLoadBean(t, &RepoRedirect{
		OwnerID:        repo.OwnerID,
		LowerName:      "oldrepo1",
		RedirectRepoID: repo.ID,
	})
}

func TestNewRepoRedirect2(t *testing.T) {
	// redirect to previously used name
	assert.NoError(t, db.PrepareTestDatabase())

	repo := db.AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.NoError(t, newRepoRedirect(db.GetEngine(db.DefaultContext), repo.OwnerID, repo.ID, repo.Name, "oldrepo1"))

	db.AssertExistsAndLoadBean(t, &RepoRedirect{
		OwnerID:        repo.OwnerID,
		LowerName:      repo.LowerName,
		RedirectRepoID: repo.ID,
	})
	db.AssertNotExistsBean(t, &RepoRedirect{
		OwnerID:        repo.OwnerID,
		LowerName:      "oldrepo1",
		RedirectRepoID: repo.ID,
	})
}

func TestNewRepoRedirect3(t *testing.T) {
	// redirect for a previously-unredirected repo
	assert.NoError(t, db.PrepareTestDatabase())

	repo := db.AssertExistsAndLoadBean(t, &Repository{ID: 2}).(*Repository)
	assert.NoError(t, newRepoRedirect(db.GetEngine(db.DefaultContext), repo.OwnerID, repo.ID, repo.Name, "newreponame"))

	db.AssertExistsAndLoadBean(t, &RepoRedirect{
		OwnerID:        repo.OwnerID,
		LowerName:      repo.LowerName,
		RedirectRepoID: repo.ID,
	})
}
