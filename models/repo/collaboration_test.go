// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestRepository_GetCollaborators(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(repoID int64) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
		collaborators, err := repo_model.GetCollaborators(db.DefaultContext, repo.ID, db.ListOptions{})
		assert.NoError(t, err)
		expectedLen, err := db.GetEngine(db.DefaultContext).Count(&repo_model.Collaboration{RepoID: repoID})
		assert.NoError(t, err)
		assert.Len(t, collaborators, int(expectedLen))
		for _, collaborator := range collaborators {
			assert.EqualValues(t, collaborator.User.ID, collaborator.Collaboration.UserID)
			assert.EqualValues(t, repoID, collaborator.Collaboration.RepoID)
		}
	}
	test(1)
	test(2)
	test(3)
	test(4)

	// Test db.ListOptions
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 22})

	collaborators1, err := repo_model.GetCollaborators(db.DefaultContext, repo.ID, db.ListOptions{PageSize: 1, Page: 1})
	assert.NoError(t, err)
	assert.Len(t, collaborators1, 1)

	collaborators2, err := repo_model.GetCollaborators(db.DefaultContext, repo.ID, db.ListOptions{PageSize: 1, Page: 2})
	assert.NoError(t, err)
	assert.Len(t, collaborators2, 1)

	assert.NotEqualValues(t, collaborators1[0].ID, collaborators2[0].ID)
}

func TestRepository_IsCollaborator(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(repoID, userID int64, expected bool) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
		actual, err := repo_model.IsCollaborator(db.DefaultContext, repo.ID, userID)
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	}
	test(3, 2, true)
	test(3, unittest.NonexistentID, false)
	test(4, 2, false)
	test(4, 4, true)
}

func TestRepository_ChangeCollaborationAccessMode(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	assert.NoError(t, repo_model.ChangeCollaborationAccessMode(db.DefaultContext, repo, 4, perm.AccessModeAdmin))

	collaboration := unittest.AssertExistsAndLoadBean(t, &repo_model.Collaboration{RepoID: repo.ID, UserID: 4})
	assert.EqualValues(t, perm.AccessModeAdmin, collaboration.Mode)

	access := unittest.AssertExistsAndLoadBean(t, &access_model.Access{UserID: 4, RepoID: repo.ID})
	assert.EqualValues(t, perm.AccessModeAdmin, access.Mode)

	assert.NoError(t, repo_model.ChangeCollaborationAccessMode(db.DefaultContext, repo, 4, perm.AccessModeAdmin))

	assert.NoError(t, repo_model.ChangeCollaborationAccessMode(db.DefaultContext, repo, unittest.NonexistentID, perm.AccessModeAdmin))

	// Disvard invalid input.
	assert.NoError(t, repo_model.ChangeCollaborationAccessMode(db.DefaultContext, repo, 4, perm.AccessMode(unittest.NonexistentID)))

	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repo.ID})
}

func TestRepository_CountCollaborators(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	count, err := repo_model.CountCollaborators(repo1.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, count)

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 22})
	count, err = repo_model.CountCollaborators(repo2.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, count)

	// Non-existent repository.
	count, err = repo_model.CountCollaborators(unittest.NonexistentID)
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)
}

func TestRepository_IsOwnerMemberCollaborator(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})

	// Organisation owner.
	actual, err := repo_model.IsOwnerMemberCollaborator(repo1, 2)
	assert.NoError(t, err)
	assert.True(t, actual)

	// Team member.
	actual, err = repo_model.IsOwnerMemberCollaborator(repo1, 4)
	assert.NoError(t, err)
	assert.True(t, actual)

	// Normal user.
	actual, err = repo_model.IsOwnerMemberCollaborator(repo1, 1)
	assert.NoError(t, err)
	assert.False(t, actual)

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})

	// Collaborator.
	actual, err = repo_model.IsOwnerMemberCollaborator(repo2, 4)
	assert.NoError(t, err)
	assert.True(t, actual)

	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 15})

	// Repository owner.
	actual, err = repo_model.IsOwnerMemberCollaborator(repo3, 2)
	assert.NoError(t, err)
	assert.True(t, actual)
}

func TestRepo_GetCollaboration(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})

	// Existing collaboration.
	collab, err := repo_model.GetCollaboration(db.DefaultContext, repo.ID, 4)
	assert.NoError(t, err)
	assert.NotNil(t, collab)
	assert.EqualValues(t, 4, collab.UserID)
	assert.EqualValues(t, 4, collab.RepoID)

	// Non-existing collaboration.
	collab, err = repo_model.GetCollaboration(db.DefaultContext, repo.ID, 1)
	assert.NoError(t, err)
	assert.Nil(t, collab)
}
