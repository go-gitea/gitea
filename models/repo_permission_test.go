// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepoPermissionPublicNonOrgRepo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// public non-organization repo
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 4}).(*Repository)
	assert.NoError(t, repo.getUnits(x))

	// plain user
	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
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
	collaborator := AssertExistsAndLoadBean(t, &User{ID: 4}).(*User)
	perm, err = GetUserRepoPermission(repo, collaborator)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// owner
	owner := AssertExistsAndLoadBean(t, &User{ID: 5}).(*User)
	perm, err = GetUserRepoPermission(repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// admin
	admin := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	perm, err = GetUserRepoPermission(repo, admin)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}
}

func TestRepoPermissionPrivateNonOrgRepo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// private non-organization repo
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 2}).(*Repository)
	assert.NoError(t, repo.getUnits(x))

	// plain user
	user := AssertExistsAndLoadBean(t, &User{ID: 4}).(*User)
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

	assert.NoError(t, repo.ChangeCollaborationAccessMode(user.ID, AccessModeRead))
	perm, err = GetUserRepoPermission(repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// owner
	owner := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	perm, err = GetUserRepoPermission(repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// admin
	admin := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	perm, err = GetUserRepoPermission(repo, admin)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}
}

func TestRepoPermissionPublicOrgRepo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// public organization repo
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 32}).(*Repository)
	assert.NoError(t, repo.getUnits(x))

	// plain user
	user := AssertExistsAndLoadBean(t, &User{ID: 5}).(*User)
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

	assert.NoError(t, repo.ChangeCollaborationAccessMode(user.ID, AccessModeRead))
	perm, err = GetUserRepoPermission(repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// org member team owner
	owner := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	perm, err = GetUserRepoPermission(repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// org member team tester
	member := AssertExistsAndLoadBean(t, &User{ID: 15}).(*User)
	perm, err = GetUserRepoPermission(repo, member)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
	}
	assert.True(t, perm.CanWrite(UnitTypeIssues))
	assert.False(t, perm.CanWrite(UnitTypeCode))

	// admin
	admin := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	perm, err = GetUserRepoPermission(repo, admin)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}
}

func TestRepoPermissionPrivateOrgRepo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// private organization repo
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 24}).(*Repository)
	assert.NoError(t, repo.getUnits(x))

	// plain user
	user := AssertExistsAndLoadBean(t, &User{ID: 5}).(*User)
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

	assert.NoError(t, repo.ChangeCollaborationAccessMode(user.ID, AccessModeRead))
	perm, err = GetUserRepoPermission(repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// org member team owner
	owner := AssertExistsAndLoadBean(t, &User{ID: 15}).(*User)
	perm, err = GetUserRepoPermission(repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// update team information and then check permission
	team := AssertExistsAndLoadBean(t, &Team{ID: 5}).(*Team)
	err = UpdateTeamUnits(team, nil)
	assert.NoError(t, err)
	perm, err = GetUserRepoPermission(repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// org member team tester
	tester := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	perm, err = GetUserRepoPermission(repo, tester)
	assert.NoError(t, err)
	assert.True(t, perm.CanWrite(UnitTypeIssues))
	assert.False(t, perm.CanWrite(UnitTypeCode))
	assert.False(t, perm.CanRead(UnitTypeCode))

	// org member team reviewer
	reviewer := AssertExistsAndLoadBean(t, &User{ID: 20}).(*User)
	perm, err = GetUserRepoPermission(repo, reviewer)
	assert.NoError(t, err)
	assert.False(t, perm.CanRead(UnitTypeIssues))
	assert.False(t, perm.CanWrite(UnitTypeCode))
	assert.True(t, perm.CanRead(UnitTypeCode))

	// admin
	admin := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	perm, err = GetUserRepoPermission(repo, admin)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}
}
