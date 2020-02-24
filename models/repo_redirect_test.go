// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLookupRepoRedirect(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	repoID, err := LookupRepoRedirect(2, "oldrepo1")
	assert.NoError(t, err)
	assert.EqualValues(t, 1, repoID)

	_, err = LookupRepoRedirect(NonexistentID, "doesnotexist")
	assert.True(t, IsErrRepoRedirectNotExist(err))
}

func TestNewRepoRedirect(t *testing.T) {
	// redirect to a completely new name
	assert.NoError(t, PrepareTestDatabase())

	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.NoError(t, NewRepoRedirect(DefaultDBContext(), repo.OwnerID, repo.ID, repo.Name, "newreponame"))

	AssertExistsAndLoadBean(t, &RepoRedirect{
		OwnerID:        repo.OwnerID,
		LowerName:      repo.LowerName,
		RedirectRepoID: repo.ID,
	})
	AssertExistsAndLoadBean(t, &RepoRedirect{
		OwnerID:        repo.OwnerID,
		LowerName:      "oldrepo1",
		RedirectRepoID: repo.ID,
	})
}

func TestNewRepoRedirect2(t *testing.T) {
	// redirect to previously used name
	assert.NoError(t, PrepareTestDatabase())

	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.NoError(t, NewRepoRedirect(DefaultDBContext(), repo.OwnerID, repo.ID, repo.Name, "oldrepo1"))

	AssertExistsAndLoadBean(t, &RepoRedirect{
		OwnerID:        repo.OwnerID,
		LowerName:      repo.LowerName,
		RedirectRepoID: repo.ID,
	})
	AssertNotExistsBean(t, &RepoRedirect{
		OwnerID:        repo.OwnerID,
		LowerName:      "oldrepo1",
		RedirectRepoID: repo.ID,
	})
}

func TestNewRepoRedirect3(t *testing.T) {
	// redirect for a previously-unredirected repo
	assert.NoError(t, PrepareTestDatabase())

	repo := AssertExistsAndLoadBean(t, &Repository{ID: 2}).(*Repository)
	assert.NoError(t, NewRepoRedirect(DefaultDBContext(), repo.OwnerID, repo.ID, repo.Name, "newreponame"))

	AssertExistsAndLoadBean(t, &RepoRedirect{
		OwnerID:        repo.OwnerID,
		LowerName:      repo.LowerName,
		RedirectRepoID: repo.ID,
	})
}
