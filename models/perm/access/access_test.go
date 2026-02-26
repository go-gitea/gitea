// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package access_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	perm_model "code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestAccessLevel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	user29 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29})
	// A public repository owned by User 2
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.False(t, repo1.IsPrivate)
	// A private repository owned by Org 3
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	assert.True(t, repo3.IsPrivate)

	// Another public repository
	repo4 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	assert.False(t, repo4.IsPrivate)
	// org. owned private repo
	repo24 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 24})

	level, err := access_model.AccessLevel(t.Context(), user2, repo1)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeOwner, level)

	level, err = access_model.AccessLevel(t.Context(), user2, repo3)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeOwner, level)

	level, err = access_model.AccessLevel(t.Context(), user5, repo1)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeRead, level)

	level, err = access_model.AccessLevel(t.Context(), user5, repo3)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeNone, level)

	// restricted user has default access to a public repo if no sign-in is required
	setting.Service.RequireSignInViewStrict = false
	level, err = access_model.AccessLevel(t.Context(), user29, repo1)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeRead, level)

	// restricted user has no access to a public repo if sign-in is required
	setting.Service.RequireSignInViewStrict = true
	level, err = access_model.AccessLevel(t.Context(), user29, repo1)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeNone, level)

	// ... unless he's a collaborator
	level, err = access_model.AccessLevel(t.Context(), user29, repo4)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeWrite, level)

	// ... or a team member
	level, err = access_model.AccessLevel(t.Context(), user29, repo24)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeRead, level)
}

func TestHasAccess(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	// A public repository owned by User 2
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.False(t, repo1.IsPrivate)
	// A private repository owned by Org 3
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	assert.True(t, repo2.IsPrivate)

	has, err := access_model.HasAnyUnitAccess(t.Context(), user1.ID, repo1)
	assert.NoError(t, err)
	assert.True(t, has)

	_, err = access_model.HasAnyUnitAccess(t.Context(), user1.ID, repo2)
	assert.NoError(t, err)

	_, err = access_model.HasAnyUnitAccess(t.Context(), user2.ID, repo1)
	assert.NoError(t, err)

	_, err = access_model.HasAnyUnitAccess(t.Context(), user2.ID, repo2)
	assert.NoError(t, err)
}

func TestRepository_RecalculateAccesses(t *testing.T) {
	// test with organization repo
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	assert.NoError(t, repo1.LoadOwner(t.Context()))

	_, err := db.GetEngine(t.Context()).Delete(&repo_model.Collaboration{UserID: 2, RepoID: 3})
	assert.NoError(t, err)
	assert.NoError(t, access_model.RecalculateAccesses(t.Context(), repo1))

	access := &access_model.Access{UserID: 2, RepoID: 3}
	has, err := db.GetEngine(t.Context()).Get(access)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, perm_model.AccessModeOwner, access.Mode)
}

func TestRepository_RecalculateAccesses2(t *testing.T) {
	// test with non-organization repo
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	assert.NoError(t, repo1.LoadOwner(t.Context()))

	_, err := db.GetEngine(t.Context()).Delete(&repo_model.Collaboration{UserID: 4, RepoID: 4})
	assert.NoError(t, err)
	assert.NoError(t, access_model.RecalculateAccesses(t.Context(), repo1))

	has, err := db.GetEngine(t.Context()).Get(&access_model.Access{UserID: 4, RepoID: 4})
	assert.NoError(t, err)
	assert.False(t, has)
}

func TestRepository_RecalculateAccesses_UpdateMode(t *testing.T) {
	// Test the update path in refreshAccesses optimization
	// Scenario: User's access mode changes from Read to Write
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	assert.NoError(t, repo.LoadOwner(t.Context()))

	// Verify initial access mode
	initialAccess := &access_model.Access{UserID: 4, RepoID: 4}
	has, err := db.GetEngine(t.Context()).Get(initialAccess)
	assert.NoError(t, err)
	assert.True(t, has)
	initialMode := initialAccess.Mode

	// Change collaboration mode to trigger update path
	newMode := perm_model.AccessModeAdmin
	assert.NotEqual(t, initialMode, newMode, "New mode should differ from initial mode")

	_, err = db.GetEngine(t.Context()).
		Where("user_id = ? AND repo_id = ?", 4, 4).
		Cols("mode").
		Update(&repo_model.Collaboration{Mode: newMode})
	assert.NoError(t, err)

	// Recalculate accesses - should UPDATE existing access, not delete+insert
	assert.NoError(t, access_model.RecalculateAccesses(t.Context(), repo))

	// Verify access was updated, not deleted and re-inserted
	updatedAccess := &access_model.Access{UserID: 4, RepoID: 4}
	has, err = db.GetEngine(t.Context()).Get(updatedAccess)
	assert.NoError(t, err)
	assert.True(t, has, "Access should still exist")
	assert.Equal(t, newMode, updatedAccess.Mode, "Access mode should be updated to new collaboration mode")
}

func TestRepository_RecalculateAccesses_RemoveAccess(t *testing.T) {
	// Test the delete path in refreshAccesses optimization
	// Scenario: Remove a user's collaboration, access should be deleted
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	assert.NoError(t, repo.LoadOwner(t.Context()))

	// Verify initial access exists
	initialAccess := &access_model.Access{UserID: 4, RepoID: 4}
	has, err := db.GetEngine(t.Context()).Get(initialAccess)
	assert.NoError(t, err)
	assert.True(t, has, "Access should exist initially")

	// Remove the collaboration to trigger delete path
	_, err = db.GetEngine(t.Context()).
		Where("user_id = ? AND repo_id = ?", 4, 4).
		Delete(&repo_model.Collaboration{})
	assert.NoError(t, err)

	// Recalculate accesses - should DELETE the access record
	assert.NoError(t, access_model.RecalculateAccesses(t.Context(), repo))

	// Verify access was deleted
	removedAccess := &access_model.Access{UserID: 4, RepoID: 4}
	has, err = db.GetEngine(t.Context()).Get(removedAccess)
	assert.NoError(t, err)
	assert.False(t, has, "Access should be deleted after removing collaboration")
}
