// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"github.com/stretchr/testify/assert"
)

func TestRepository_AddCollaborator(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	testSuccess := func(repoID, userID int64) {
		repo := db.AssertExistsAndLoadBean(t, &Repository{ID: repoID}).(*Repository)
		assert.NoError(t, repo.GetOwner())
		user := db.AssertExistsAndLoadBean(t, &User{ID: userID}).(*User)
		assert.NoError(t, repo.AddCollaborator(user))
		CheckConsistencyFor(t, &Repository{ID: repoID}, &User{ID: userID})
	}
	testSuccess(1, 4)
	testSuccess(1, 4)
	testSuccess(3, 4)
}

func TestRepository_GetCollaborators(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())
	test := func(repoID int64) {
		repo := db.AssertExistsAndLoadBean(t, &Repository{ID: repoID}).(*Repository)
		collaborators, err := repo.GetCollaborators(db.ListOptions{})
		assert.NoError(t, err)
		expectedLen, err := db.GetEngine(db.DefaultContext).Count(&Collaboration{RepoID: repoID})
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
	assert.NoError(t, db.PrepareTestDatabase())

	test := func(repoID, userID int64, expected bool) {
		repo := db.AssertExistsAndLoadBean(t, &Repository{ID: repoID}).(*Repository)
		actual, err := repo.IsCollaborator(userID)
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	}
	test(3, 2, true)
	test(3, db.NonexistentID, false)
	test(4, 2, false)
	test(4, 4, true)
}

func TestRepository_ChangeCollaborationAccessMode(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	repo := db.AssertExistsAndLoadBean(t, &Repository{ID: 4}).(*Repository)
	assert.NoError(t, repo.ChangeCollaborationAccessMode(4, AccessModeAdmin))

	collaboration := db.AssertExistsAndLoadBean(t, &Collaboration{RepoID: repo.ID, UserID: 4}).(*Collaboration)
	assert.EqualValues(t, AccessModeAdmin, collaboration.Mode)

	access := db.AssertExistsAndLoadBean(t, &Access{UserID: 4, RepoID: repo.ID}).(*Access)
	assert.EqualValues(t, AccessModeAdmin, access.Mode)

	assert.NoError(t, repo.ChangeCollaborationAccessMode(4, AccessModeAdmin))

	assert.NoError(t, repo.ChangeCollaborationAccessMode(db.NonexistentID, AccessModeAdmin))

	CheckConsistencyFor(t, &Repository{ID: repo.ID})
}

func TestRepository_DeleteCollaboration(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	repo := db.AssertExistsAndLoadBean(t, &Repository{ID: 4}).(*Repository)
	assert.NoError(t, repo.GetOwner())
	assert.NoError(t, repo.DeleteCollaboration(4))
	db.AssertNotExistsBean(t, &Collaboration{RepoID: repo.ID, UserID: 4})

	assert.NoError(t, repo.DeleteCollaboration(4))
	db.AssertNotExistsBean(t, &Collaboration{RepoID: repo.ID, UserID: 4})

	CheckConsistencyFor(t, &Repository{ID: repo.ID})
}
