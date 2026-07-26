// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanUserDelete(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Personal repo: owner can delete, unrelated user cannot
	personalRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: personalRepo.OwnerID})
	stranger := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	can, err := CanUserDelete(t.Context(), personalRepo, owner)
	require.NoError(t, err)
	assert.True(t, can, "personal repo owner should be able to delete")

	can, err = CanUserDelete(t.Context(), personalRepo, stranger)
	require.NoError(t, err)
	assert.False(t, can, "unrelated user should not delete personal repo")

	// Site admin can always delete
	siteAdmin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	can, err = CanUserDelete(t.Context(), personalRepo, siteAdmin)
	require.NoError(t, err)
	assert.True(t, can, "site admin should be able to delete")

	// Org repo: write collaborator cannot delete; admin collaborator can
	// (admin access is what org-repo creators receive — see create.go)
	orgRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}) // org3/repo3
	// user5 is not an org3 owner/admin; use a clean collaborator on repo3
	collab := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	can, err = CanUserDelete(t.Context(), orgRepo, collab)
	require.NoError(t, err)
	assert.False(t, can, "non-admin user should not delete org repo")

	// Grant write only — still cannot delete
	require.NoError(t, db.Insert(t.Context(), &repo_model.Collaboration{
		RepoID: orgRepo.ID,
		UserID: collab.ID,
		Mode:   perm.AccessModeWrite,
	}))
	require.NoError(t, access_model.RecalculateUserAccess(t.Context(), orgRepo, collab.ID))

	can, err = CanUserDelete(t.Context(), orgRepo, collab)
	require.NoError(t, err)
	assert.False(t, can, "write-only collaborator should not delete org repo")

	// Promote to repo admin — can delete (matches creator-gets-admin behavior)
	_, err = db.GetEngine(t.Context()).
		Where("repo_id=? AND user_id=?", orgRepo.ID, collab.ID).
		Cols("mode").
		Update(&repo_model.Collaboration{Mode: perm.AccessModeAdmin})
	require.NoError(t, err)
	require.NoError(t, access_model.RecalculateUserAccess(t.Context(), orgRepo, collab.ID))

	can, err = CanUserDelete(t.Context(), orgRepo, collab)
	require.NoError(t, err)
	assert.True(t, can, "repo admin (org-repo creator style) should be able to delete")
}
