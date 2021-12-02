// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	perm_model "code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestRepoPermissionPublicNonOrgRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// public non-organization repo
	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 4}).(*Repository)
	assert.NoError(t, repo.getUnits(db.GetEngine(db.DefaultContext)))

	// plain user
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	perm, err := GetUserRepoPermission(repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// change to collaborator
	assert.NoError(t, repo.AddCollaborator(user))
	perm, err = GetUserRepoPermission(repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// collaborator
	collaborator := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}).(*user_model.User)
	perm, err = GetUserRepoPermission(repo, collaborator)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// owner
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}).(*user_model.User)
	perm, err = GetUserRepoPermission(repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// admin
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	perm, err = GetUserRepoPermission(repo, admin)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}
}

func TestRepoPermissionPrivateNonOrgRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// private non-organization repo
	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 2}).(*Repository)
	assert.NoError(t, repo.getUnits(db.GetEngine(db.DefaultContext)))

	// plain user
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}).(*user_model.User)
	perm, err := GetUserRepoPermission(repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.False(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// change to collaborator to default write access
	assert.NoError(t, repo.AddCollaborator(user))
	perm, err = GetUserRepoPermission(repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	assert.NoError(t, repo.ChangeCollaborationAccessMode(user.ID, perm_model.AccessModeRead))
	perm, err = GetUserRepoPermission(repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// owner
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	perm, err = GetUserRepoPermission(repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// admin
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	perm, err = GetUserRepoPermission(repo, admin)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}
}

func TestRepoPermissionPublicOrgRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// public organization repo
	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 32}).(*Repository)
	assert.NoError(t, repo.getUnits(db.GetEngine(db.DefaultContext)))

	// plain user
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}).(*user_model.User)
	perm, err := GetUserRepoPermission(repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// change to collaborator to default write access
	assert.NoError(t, repo.AddCollaborator(user))
	perm, err = GetUserRepoPermission(repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	assert.NoError(t, repo.ChangeCollaborationAccessMode(user.ID, perm_model.AccessModeRead))
	perm, err = GetUserRepoPermission(repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// org member team owner
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	perm, err = GetUserRepoPermission(repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// org member team tester
	member := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15}).(*user_model.User)
	perm, err = GetUserRepoPermission(repo, member)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
	}
	assert.True(t, perm.CanWrite(unit.TypeIssues))
	assert.False(t, perm.CanWrite(unit.TypeCode))

	// admin
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	perm, err = GetUserRepoPermission(repo, admin)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}
}

func TestRepoPermissionPrivateOrgRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// private organization repo
	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 24}).(*Repository)
	assert.NoError(t, repo.getUnits(db.GetEngine(db.DefaultContext)))

	// plain user
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}).(*user_model.User)
	perm, err := GetUserRepoPermission(repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.False(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// change to collaborator to default write access
	assert.NoError(t, repo.AddCollaborator(user))
	perm, err = GetUserRepoPermission(repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	assert.NoError(t, repo.ChangeCollaborationAccessMode(user.ID, perm_model.AccessModeRead))
	perm, err = GetUserRepoPermission(repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// org member team owner
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15}).(*user_model.User)
	perm, err = GetUserRepoPermission(repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// update team information and then check permission
	team := unittest.AssertExistsAndLoadBean(t, &Team{ID: 5}).(*Team)
	err = UpdateTeamUnits(team, nil)
	assert.NoError(t, err)
	perm, err = GetUserRepoPermission(repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// org member team tester
	tester := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	perm, err = GetUserRepoPermission(repo, tester)
	assert.NoError(t, err)
	assert.True(t, perm.CanWrite(unit.TypeIssues))
	assert.False(t, perm.CanWrite(unit.TypeCode))
	assert.False(t, perm.CanRead(unit.TypeCode))

	// org member team reviewer
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 20}).(*user_model.User)
	perm, err = GetUserRepoPermission(repo, reviewer)
	assert.NoError(t, err)
	assert.False(t, perm.CanRead(unit.TypeIssues))
	assert.False(t, perm.CanWrite(unit.TypeCode))
	assert.True(t, perm.CanRead(unit.TypeCode))

	// admin
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	perm, err = GetUserRepoPermission(repo, admin)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}
}
