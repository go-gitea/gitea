// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepository_AddCollaborator(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testSuccess := func(repoID, userID int64) {
		repo := AssertExistsAndLoadBean(t, &Repository{ID: repoID}).(*Repository)
		assert.NoError(t, repo.GetOwner())
		user := AssertExistsAndLoadBean(t, &User{ID: userID}).(*User)
		assert.NoError(t, repo.AddCollaborator(user))
		CheckConsistencyFor(t, &Repository{ID: repoID}, &User{ID: userID})
	}
	testSuccess(1, 4)
	testSuccess(1, 4)
	testSuccess(3, 4)
}

func TestRepository_GetCollaborators(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	test := func(repoID int64) {
		repo := AssertExistsAndLoadBean(t, &Repository{ID: repoID}).(*Repository)
		collaborators, err := repo.GetCollaborators()
		assert.NoError(t, err)
		expectedLen, err := x.Count(&Collaboration{RepoID: repoID})
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
}

func TestRepository_IsCollaborator(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	test := func(repoID, userID int64, expected bool) {
		repo := AssertExistsAndLoadBean(t, &Repository{ID: repoID}).(*Repository)
		actual, err := repo.IsCollaborator(userID)
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	}
	test(3, 2, true)
	test(3, NonexistentID, false)
	test(4, 2, false)
	test(4, 4, true)
}

func TestRepository_ChangeCollaborationAccessMode(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	repo := AssertExistsAndLoadBean(t, &Repository{ID: 4}).(*Repository)
	assert.NoError(t, repo.ChangeCollaborationAccessMode(4, AccessModeAdmin))

	collaboration := AssertExistsAndLoadBean(t, &Collaboration{RepoID: repo.ID, UserID: 4}).(*Collaboration)
	assert.EqualValues(t, AccessModeAdmin, collaboration.Mode)

	access := AssertExistsAndLoadBean(t, &Access{UserID: 4, RepoID: repo.ID}).(*Access)
	assert.EqualValues(t, AccessModeAdmin, access.Mode)

	assert.NoError(t, repo.ChangeCollaborationAccessMode(4, AccessModeAdmin))

	assert.NoError(t, repo.ChangeCollaborationAccessMode(NonexistentID, AccessModeAdmin))

	CheckConsistencyFor(t, &Repository{ID: repo.ID})
}

func TestRepository_DeleteCollaboration(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	repo := AssertExistsAndLoadBean(t, &Repository{ID: 4}).(*Repository)
	assert.NoError(t, repo.GetOwner())
	assert.NoError(t, repo.DeleteCollaboration(4))
	AssertNotExistsBean(t, &Collaboration{RepoID: repo.ID, UserID: 4})

	assert.NoError(t, repo.DeleteCollaboration(4))
	AssertNotExistsBean(t, &Collaboration{RepoID: repo.ID, UserID: 4})

	CheckConsistencyFor(t, &Repository{ID: repo.ID})
}
