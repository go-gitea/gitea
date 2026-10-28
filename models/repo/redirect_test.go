// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestLookupRedirect(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repoID, err := repo_model.LookupRedirect(t.Context(), 2, "oldrepo1")
	assert.NoError(t, err)
	assert.EqualValues(t, 1, repoID)

	_, err = repo_model.LookupRedirect(t.Context(), unittest.NonexistentID, "doesnotexist")
	assert.True(t, repo_model.IsErrRedirectNotExist(err))
}

func TestNewRedirect(t *testing.T) {
	for _, tc := range []struct {
		repoID  int64
		newName string
	}{
		{1, "newreponame"}, // redirect to a completely new name
		{1, "oldrepo1"},    // redirect to previously used name
		{2, "newreponame"}, // redirect for a previously-unredirected repo
	} {
		t.Run(tc.newName, func(t *testing.T) {
			assert.NoError(t, unittest.PrepareTestDatabase())

			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: tc.repoID})
			assert.NoError(t, repo_model.NewRedirect(t.Context(), repo.OwnerID, repo.ID, repo.Name, tc.newName))

			unittest.AssertExistsAndLoadBean(t, &repo_model.Redirect{
				OwnerID:        repo.OwnerID,
				LowerName:      repo.LowerName,
				RedirectRepoID: repo.ID,
			})
			if tc.repoID == 1 {
				redirect := &repo_model.Redirect{
					OwnerID:        repo.OwnerID,
					LowerName:      "oldrepo1",
					RedirectRepoID: repo.ID,
				}
				if tc.newName == "oldrepo1" {
					unittest.AssertNotExistsBean(t, redirect)
				} else {
					unittest.AssertExistsAndLoadBean(t, redirect)
				}
			}
		})
	}
}
