// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package access_test

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	perm_model "code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestAccessLevel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}).(*user_model.User)
	user29 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29}).(*user_model.User)
	// A public repository owned by User 2
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	assert.False(t, repo1.IsPrivate)
	// A private repository owned by Org 3
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}).(*repo_model.Repository)
	assert.True(t, repo3.IsPrivate)

	// Another public repository
	repo4 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4}).(*repo_model.Repository)
	assert.False(t, repo4.IsPrivate)
	// org. owned private repo
	repo24 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 24}).(*repo_model.Repository)

	level, err := access_model.AccessLevel(user2, repo1)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeOwner, level)

	level, err = access_model.AccessLevel(user2, repo3)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeOwner, level)

	level, err = access_model.AccessLevel(user5, repo1)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeRead, level)

	level, err = access_model.AccessLevel(user5, repo3)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeNone, level)

	// restricted user has no access to a public repo
	level, err = access_model.AccessLevel(user29, repo1)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeNone, level)

	// ... unless he's a collaborator
	level, err = access_model.AccessLevel(user29, repo4)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeWrite, level)

	// ... or a team member
	level, err = access_model.AccessLevel(user29, repo24)
	assert.NoError(t, err)
	assert.Equal(t, perm_model.AccessModeRead, level)
}

func TestHasAccess(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}).(*user_model.User)
	// A public repository owned by User 2
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	assert.False(t, repo1.IsPrivate)
	// A private repository owned by Org 3
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}).(*repo_model.Repository)
	assert.True(t, repo2.IsPrivate)

	has, err := access_model.HasAccess(db.DefaultContext, user1.ID, repo1)
	assert.NoError(t, err)
	assert.True(t, has)

	_, err = access_model.HasAccess(db.DefaultContext, user1.ID, repo2)
	assert.NoError(t, err)

	_, err = access_model.HasAccess(db.DefaultContext, user2.ID, repo1)
	assert.NoError(t, err)

	_, err = access_model.HasAccess(db.DefaultContext, user2.ID, repo2)
	assert.NoError(t, err)
}

func TestRepository_RecalculateAccesses(t *testing.T) {
	// test with organization repo
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}).(*repo_model.Repository)
	assert.NoError(t, repo1.GetOwner(db.DefaultContext))

	_, err := db.GetEngine(db.DefaultContext).Delete(&repo_model.Collaboration{UserID: 2, RepoID: 3})
	assert.NoError(t, err)
	assert.NoError(t, access_model.RecalculateAccesses(db.DefaultContext, repo1))

	access := &access_model.Access{UserID: 2, RepoID: 3}
	has, err := db.GetEngine(db.DefaultContext).Get(access)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, perm_model.AccessModeOwner, access.Mode)
}

func TestRepository_RecalculateAccesses2(t *testing.T) {
	// test with non-organization repo
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4}).(*repo_model.Repository)
	assert.NoError(t, repo1.GetOwner(db.DefaultContext))

	_, err := db.GetEngine(db.DefaultContext).Delete(&repo_model.Collaboration{UserID: 4, RepoID: 4})
	assert.NoError(t, err)
	assert.NoError(t, access_model.RecalculateAccesses(db.DefaultContext, repo1))

	has, err := db.GetEngine(db.DefaultContext).Get(&access_model.Access{UserID: 4, RepoID: 4})
	assert.NoError(t, err)
	assert.False(t, has)
}

func TestRepoPermissionPublicNonOrgRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// public non-organization repo
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4}).(*repo_model.Repository)
	assert.NoError(t, repo.LoadUnits(db.DefaultContext))

	// plain user
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	perm, err := access_model.GetUserRepoPermission(db.DefaultContext, repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// change to collaborator
	assert.NoError(t, models.AddCollaborator(repo, user))
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// collaborator
	collaborator := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}).(*user_model.User)
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, collaborator)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// owner
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}).(*user_model.User)
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// admin
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, admin)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}
}

func TestRepoPermissionPrivateNonOrgRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// private non-organization repo
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2}).(*repo_model.Repository)
	assert.NoError(t, repo.LoadUnits(db.DefaultContext))

	// plain user
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}).(*user_model.User)
	perm, err := access_model.GetUserRepoPermission(db.DefaultContext, repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.False(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// change to collaborator to default write access
	assert.NoError(t, models.AddCollaborator(repo, user))
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	assert.NoError(t, repo_model.ChangeCollaborationAccessMode(repo, user.ID, perm_model.AccessModeRead))
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// owner
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// admin
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, admin)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}
}

func TestRepoPermissionPublicOrgRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// public organization repo
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 32}).(*repo_model.Repository)
	assert.NoError(t, repo.LoadUnits(db.DefaultContext))

	// plain user
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}).(*user_model.User)
	perm, err := access_model.GetUserRepoPermission(db.DefaultContext, repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// change to collaborator to default write access
	assert.NoError(t, models.AddCollaborator(repo, user))
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	assert.NoError(t, repo_model.ChangeCollaborationAccessMode(repo, user.ID, perm_model.AccessModeRead))
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// org member team owner
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// org member team tester
	member := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15}).(*user_model.User)
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, member)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
	}
	assert.True(t, perm.CanWrite(unit.TypeIssues))
	assert.False(t, perm.CanWrite(unit.TypeCode))

	// admin
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, admin)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}
}

func TestRepoPermissionPrivateOrgRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// private organization repo
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 24}).(*repo_model.Repository)
	assert.NoError(t, repo.LoadUnits(db.DefaultContext))

	// plain user
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}).(*user_model.User)
	perm, err := access_model.GetUserRepoPermission(db.DefaultContext, repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.False(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// change to collaborator to default write access
	assert.NoError(t, models.AddCollaborator(repo, user))
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	assert.NoError(t, repo_model.ChangeCollaborationAccessMode(repo, user.ID, perm_model.AccessModeRead))
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, user)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.False(t, perm.CanWrite(unit.Type))
	}

	// org member team owner
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15}).(*user_model.User)
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// update team information and then check permission
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 5}).(*organization.Team)
	err = organization.UpdateTeamUnits(team, nil)
	assert.NoError(t, err)
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, owner)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}

	// org member team tester
	tester := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, tester)
	assert.NoError(t, err)
	assert.True(t, perm.CanWrite(unit.TypeIssues))
	assert.False(t, perm.CanWrite(unit.TypeCode))
	assert.False(t, perm.CanRead(unit.TypeCode))

	// org member team reviewer
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 20}).(*user_model.User)
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, reviewer)
	assert.NoError(t, err)
	assert.False(t, perm.CanRead(unit.TypeIssues))
	assert.False(t, perm.CanWrite(unit.TypeCode))
	assert.True(t, perm.CanRead(unit.TypeCode))

	// admin
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	perm, err = access_model.GetUserRepoPermission(db.DefaultContext, repo, admin)
	assert.NoError(t, err)
	for _, unit := range repo.Units {
		assert.True(t, perm.CanRead(unit.Type))
		assert.True(t, perm.CanWrite(unit.Type))
	}
}
