// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestUser_IsOwnedBy(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	for _, testCase := range []struct {
		OrgID         int64
		UserID        int64
		ExpectedOwner bool
	}{
		{3, 2, true},
		{3, 1, false},
		{3, 3, false},
		{3, 4, false},
		{2, 2, false}, // user2 is not an organization
		{2, 3, false},
	} {
		org := db.AssertExistsAndLoadBean(t, &User{ID: testCase.OrgID}).(*User)
		isOwner, err := org.IsOwnedBy(testCase.UserID)
		assert.NoError(t, err)
		assert.Equal(t, testCase.ExpectedOwner, isOwner)
	}
}

func TestUser_IsOrgMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	for _, testCase := range []struct {
		OrgID          int64
		UserID         int64
		ExpectedMember bool
	}{
		{3, 2, true},
		{3, 4, true},
		{3, 1, false},
		{3, 3, false},
		{2, 2, false}, // user2 is not an organization
		{2, 3, false},
	} {
		org := db.AssertExistsAndLoadBean(t, &User{ID: testCase.OrgID}).(*User)
		isMember, err := org.IsOrgMember(testCase.UserID)
		assert.NoError(t, err)
		assert.Equal(t, testCase.ExpectedMember, isMember)
	}
}

func TestUser_GetTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	team, err := org.GetTeam("team1")
	assert.NoError(t, err)
	assert.Equal(t, org.ID, team.OrgID)
	assert.Equal(t, "team1", team.LowerName)

	_, err = org.GetTeam("does not exist")
	assert.True(t, IsErrTeamNotExist(err))

	nonOrg := db.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	_, err = nonOrg.GetTeam("team")
	assert.True(t, IsErrTeamNotExist(err))
}

func TestUser_GetOwnerTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	team, err := org.GetOwnerTeam()
	assert.NoError(t, err)
	assert.Equal(t, org.ID, team.OrgID)

	nonOrg := db.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	_, err = nonOrg.GetOwnerTeam()
	assert.True(t, IsErrTeamNotExist(err))
}

func TestUser_GetTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	assert.NoError(t, org.LoadTeams())
	if assert.Len(t, org.Teams, 4) {
		assert.Equal(t, int64(1), org.Teams[0].ID)
		assert.Equal(t, int64(2), org.Teams[1].ID)
		assert.Equal(t, int64(12), org.Teams[2].ID)
		assert.Equal(t, int64(7), org.Teams[3].ID)
	}
}

func TestUser_GetMembers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	assert.NoError(t, org.GetMembers())
	if assert.Len(t, org.Members, 3) {
		assert.Equal(t, int64(2), org.Members[0].ID)
		assert.Equal(t, int64(28), org.Members[1].ID)
		assert.Equal(t, int64(4), org.Members[2].ID)
	}
}

func TestUser_AddMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)

	// add a user that is not a member
	db.AssertNotExistsBean(t, &OrgUser{UID: 5, OrgID: 3})
	prevNumMembers := org.NumMembers
	assert.NoError(t, org.AddMember(5))
	db.AssertExistsAndLoadBean(t, &OrgUser{UID: 5, OrgID: 3})
	org = db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	assert.Equal(t, prevNumMembers+1, org.NumMembers)

	// add a user that is already a member
	db.AssertExistsAndLoadBean(t, &OrgUser{UID: 4, OrgID: 3})
	prevNumMembers = org.NumMembers
	assert.NoError(t, org.AddMember(4))
	db.AssertExistsAndLoadBean(t, &OrgUser{UID: 4, OrgID: 3})
	org = db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	assert.Equal(t, prevNumMembers, org.NumMembers)

	CheckConsistencyFor(t, &User{})
}

func TestUser_RemoveMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)

	// remove a user that is a member
	db.AssertExistsAndLoadBean(t, &OrgUser{UID: 4, OrgID: 3})
	prevNumMembers := org.NumMembers
	assert.NoError(t, org.RemoveMember(4))
	db.AssertNotExistsBean(t, &OrgUser{UID: 4, OrgID: 3})
	org = db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	assert.Equal(t, prevNumMembers-1, org.NumMembers)

	// remove a user that is not a member
	db.AssertNotExistsBean(t, &OrgUser{UID: 5, OrgID: 3})
	prevNumMembers = org.NumMembers
	assert.NoError(t, org.RemoveMember(5))
	db.AssertNotExistsBean(t, &OrgUser{UID: 5, OrgID: 3})
	org = db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	assert.Equal(t, prevNumMembers, org.NumMembers)

	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestUser_RemoveOrgRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	repo := db.AssertExistsAndLoadBean(t, &Repository{OwnerID: org.ID}).(*Repository)

	// remove a repo that does belong to org
	db.AssertExistsAndLoadBean(t, &TeamRepo{RepoID: repo.ID, OrgID: org.ID})
	assert.NoError(t, org.RemoveOrgRepo(repo.ID))
	db.AssertNotExistsBean(t, &TeamRepo{RepoID: repo.ID, OrgID: org.ID})
	db.AssertExistsAndLoadBean(t, &Repository{ID: repo.ID}) // repo should still exist

	// remove a repo that does not belong to org
	assert.NoError(t, org.RemoveOrgRepo(repo.ID))
	db.AssertNotExistsBean(t, &TeamRepo{RepoID: repo.ID, OrgID: org.ID})

	assert.NoError(t, org.RemoveOrgRepo(db.NonexistentID))

	CheckConsistencyFor(t,
		&User{ID: org.ID},
		&Team{OrgID: org.ID},
		&Repository{ID: repo.ID})
}

func TestCreateOrganization(t *testing.T) {
	// successful creation of org
	assert.NoError(t, unittest.PrepareTestDatabase())

	owner := db.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	const newOrgName = "neworg"
	org := &User{
		Name: newOrgName,
	}

	db.AssertNotExistsBean(t, &User{Name: newOrgName, Type: UserTypeOrganization})
	assert.NoError(t, CreateOrganization(org, owner))
	org = db.AssertExistsAndLoadBean(t,
		&User{Name: newOrgName, Type: UserTypeOrganization}).(*User)
	ownerTeam := db.AssertExistsAndLoadBean(t,
		&Team{Name: ownerTeamName, OrgID: org.ID}).(*Team)
	db.AssertExistsAndLoadBean(t, &TeamUser{UID: owner.ID, TeamID: ownerTeam.ID})
	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestCreateOrganization2(t *testing.T) {
	// unauthorized creation of org
	assert.NoError(t, unittest.PrepareTestDatabase())

	owner := db.AssertExistsAndLoadBean(t, &User{ID: 5}).(*User)
	const newOrgName = "neworg"
	org := &User{
		Name: newOrgName,
	}

	db.AssertNotExistsBean(t, &User{Name: newOrgName, Type: UserTypeOrganization})
	err := CreateOrganization(org, owner)
	assert.Error(t, err)
	assert.True(t, IsErrUserNotAllowedCreateOrg(err))
	db.AssertNotExistsBean(t, &User{Name: newOrgName, Type: UserTypeOrganization})
	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestCreateOrganization3(t *testing.T) {
	// create org with same name as existent org
	assert.NoError(t, unittest.PrepareTestDatabase())

	owner := db.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	org := &User{Name: "user3"}                          // should already exist
	db.AssertExistsAndLoadBean(t, &User{Name: org.Name}) // sanity check
	err := CreateOrganization(org, owner)
	assert.Error(t, err)
	assert.True(t, IsErrUserAlreadyExist(err))
	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestCreateOrganization4(t *testing.T) {
	// create org with unusable name
	assert.NoError(t, unittest.PrepareTestDatabase())

	owner := db.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	err := CreateOrganization(&User{Name: "assets"}, owner)
	assert.Error(t, err)
	assert.True(t, IsErrNameReserved(err))
	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestGetOrgByName(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	org, err := GetOrgByName("user3")
	assert.NoError(t, err)
	assert.EqualValues(t, 3, org.ID)
	assert.Equal(t, "user3", org.Name)

	_, err = GetOrgByName("user2") // user2 is an individual
	assert.True(t, IsErrOrgNotExist(err))

	_, err = GetOrgByName("") // corner case
	assert.True(t, IsErrOrgNotExist(err))
}

func TestCountOrganizations(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	expected, err := db.GetEngine(db.DefaultContext).Where("type=?", UserTypeOrganization).Count(&User{})
	assert.NoError(t, err)
	assert.Equal(t, expected, CountOrganizations())
}

func TestDeleteOrganization(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := db.AssertExistsAndLoadBean(t, &User{ID: 6}).(*User)
	assert.NoError(t, DeleteOrganization(org))
	db.AssertNotExistsBean(t, &User{ID: 6})
	db.AssertNotExistsBean(t, &OrgUser{OrgID: 6})
	db.AssertNotExistsBean(t, &Team{OrgID: 6})

	org = db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	err := DeleteOrganization(org)
	assert.Error(t, err)
	assert.True(t, IsErrUserOwnRepos(err))

	user := db.AssertExistsAndLoadBean(t, &User{ID: 5}).(*User)
	assert.Error(t, DeleteOrganization(user))
	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestIsOrganizationOwner(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(orgID, userID int64, expected bool) {
		isOwner, err := IsOrganizationOwner(orgID, userID)
		assert.NoError(t, err)
		assert.EqualValues(t, expected, isOwner)
	}
	test(3, 2, true)
	test(3, 3, false)
	test(6, 5, true)
	test(6, 4, false)
	test(db.NonexistentID, db.NonexistentID, false)
}

func TestIsOrganizationMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(orgID, userID int64, expected bool) {
		isMember, err := IsOrganizationMember(orgID, userID)
		assert.NoError(t, err)
		assert.EqualValues(t, expected, isMember)
	}
	test(3, 2, true)
	test(3, 3, false)
	test(3, 4, true)
	test(6, 5, true)
	test(6, 4, false)
	test(db.NonexistentID, db.NonexistentID, false)
}

func TestIsPublicMembership(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(orgID, userID int64, expected bool) {
		isMember, err := IsPublicMembership(orgID, userID)
		assert.NoError(t, err)
		assert.EqualValues(t, expected, isMember)
	}
	test(3, 2, true)
	test(3, 3, false)
	test(3, 4, false)
	test(6, 5, true)
	test(6, 4, false)
	test(db.NonexistentID, db.NonexistentID, false)
}

func TestGetOrgsByUserID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	orgs, err := GetOrgsByUserID(4, true)
	assert.NoError(t, err)
	if assert.Len(t, orgs, 1) {
		assert.EqualValues(t, 3, orgs[0].ID)
	}

	orgs, err = GetOrgsByUserID(4, false)
	assert.NoError(t, err)
	assert.Len(t, orgs, 0)
}

func TestGetOwnedOrgsByUserID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	orgs, err := GetOwnedOrgsByUserID(2)
	assert.NoError(t, err)
	if assert.Len(t, orgs, 1) {
		assert.EqualValues(t, 3, orgs[0].ID)
	}

	orgs, err = GetOwnedOrgsByUserID(4)
	assert.NoError(t, err)
	assert.Len(t, orgs, 0)
}

func TestGetOwnedOrgsByUserIDDesc(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	orgs, err := GetOwnedOrgsByUserIDDesc(5, "id")
	assert.NoError(t, err)
	if assert.Len(t, orgs, 2) {
		assert.EqualValues(t, 7, orgs[0].ID)
		assert.EqualValues(t, 6, orgs[1].ID)
	}

	orgs, err = GetOwnedOrgsByUserIDDesc(4, "id")
	assert.NoError(t, err)
	assert.Len(t, orgs, 0)
}

func TestGetOrgUsersByUserID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	orgUsers, err := GetOrgUsersByUserID(5, &SearchOrganizationsOptions{All: true})
	assert.NoError(t, err)
	if assert.Len(t, orgUsers, 2) {
		assert.Equal(t, OrgUser{
			ID:       orgUsers[0].ID,
			OrgID:    6,
			UID:      5,
			IsPublic: true,
		}, *orgUsers[0])
		assert.Equal(t, OrgUser{
			ID:       orgUsers[1].ID,
			OrgID:    7,
			UID:      5,
			IsPublic: false,
		}, *orgUsers[1])
	}

	publicOrgUsers, err := GetOrgUsersByUserID(5, &SearchOrganizationsOptions{All: false})
	assert.NoError(t, err)
	assert.Len(t, publicOrgUsers, 1)
	assert.Equal(t, *orgUsers[0], *publicOrgUsers[0])

	orgUsers, err = GetOrgUsersByUserID(1, &SearchOrganizationsOptions{All: true})
	assert.NoError(t, err)
	assert.Len(t, orgUsers, 0)
}

func TestGetOrgUsersByOrgID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	orgUsers, err := GetOrgUsersByOrgID(&FindOrgMembersOpts{
		ListOptions: db.ListOptions{},
		OrgID:       3,
		PublicOnly:  false,
	})
	assert.NoError(t, err)
	if assert.Len(t, orgUsers, 3) {
		assert.Equal(t, OrgUser{
			ID:       orgUsers[0].ID,
			OrgID:    3,
			UID:      2,
			IsPublic: true,
		}, *orgUsers[0])
		assert.Equal(t, OrgUser{
			ID:       orgUsers[1].ID,
			OrgID:    3,
			UID:      4,
			IsPublic: false,
		}, *orgUsers[1])
	}

	orgUsers, err = GetOrgUsersByOrgID(&FindOrgMembersOpts{
		ListOptions: db.ListOptions{},
		OrgID:       db.NonexistentID,
		PublicOnly:  false,
	})
	assert.NoError(t, err)
	assert.Len(t, orgUsers, 0)
}

func TestChangeOrgUserStatus(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(orgID, userID int64, public bool) {
		assert.NoError(t, ChangeOrgUserStatus(orgID, userID, public))
		orgUser := db.AssertExistsAndLoadBean(t, &OrgUser{OrgID: orgID, UID: userID}).(*OrgUser)
		assert.Equal(t, public, orgUser.IsPublic)
	}

	testSuccess(3, 2, false)
	testSuccess(3, 2, false)
	testSuccess(3, 4, true)
	assert.NoError(t, ChangeOrgUserStatus(db.NonexistentID, db.NonexistentID, true))
}

func TestAddOrgUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	testSuccess := func(orgID, userID int64, isPublic bool) {
		org := db.AssertExistsAndLoadBean(t, &User{ID: orgID}).(*User)
		expectedNumMembers := org.NumMembers
		if !db.BeanExists(t, &OrgUser{OrgID: orgID, UID: userID}) {
			expectedNumMembers++
		}
		assert.NoError(t, AddOrgUser(orgID, userID))
		ou := &OrgUser{OrgID: orgID, UID: userID}
		db.AssertExistsAndLoadBean(t, ou)
		assert.Equal(t, isPublic, ou.IsPublic)
		org = db.AssertExistsAndLoadBean(t, &User{ID: orgID}).(*User)
		assert.EqualValues(t, expectedNumMembers, org.NumMembers)
	}

	setting.Service.DefaultOrgMemberVisible = false
	testSuccess(3, 5, false)
	testSuccess(3, 5, false)
	testSuccess(6, 2, false)

	setting.Service.DefaultOrgMemberVisible = true
	testSuccess(6, 3, true)

	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestRemoveOrgUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	testSuccess := func(orgID, userID int64) {
		org := db.AssertExistsAndLoadBean(t, &User{ID: orgID}).(*User)
		expectedNumMembers := org.NumMembers
		if db.BeanExists(t, &OrgUser{OrgID: orgID, UID: userID}) {
			expectedNumMembers--
		}
		assert.NoError(t, RemoveOrgUser(orgID, userID))
		db.AssertNotExistsBean(t, &OrgUser{OrgID: orgID, UID: userID})
		org = db.AssertExistsAndLoadBean(t, &User{ID: orgID}).(*User)
		assert.EqualValues(t, expectedNumMembers, org.NumMembers)
	}
	testSuccess(3, 4)
	testSuccess(3, 4)

	err := RemoveOrgUser(7, 5)
	assert.Error(t, err)
	assert.True(t, IsErrLastOrgOwner(err))
	db.AssertExistsAndLoadBean(t, &OrgUser{OrgID: 7, UID: 5})
	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestUser_GetUserTeamIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	testSuccess := func(userID int64, expected []int64) {
		teamIDs, err := org.GetUserTeamIDs(userID)
		assert.NoError(t, err)
		assert.Equal(t, expected, teamIDs)
	}
	testSuccess(2, []int64{1, 2})
	testSuccess(4, []int64{2})
	testSuccess(db.NonexistentID, []int64{})
}

func TestAccessibleReposEnv_CountRepos(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	testSuccess := func(userID, expectedCount int64) {
		env, err := org.AccessibleReposEnv(userID)
		assert.NoError(t, err)
		count, err := env.CountRepos()
		assert.NoError(t, err)
		assert.EqualValues(t, expectedCount, count)
	}
	testSuccess(2, 3)
	testSuccess(4, 2)
}

func TestAccessibleReposEnv_RepoIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	testSuccess := func(userID, _, pageSize int64, expectedRepoIDs []int64) {
		env, err := org.AccessibleReposEnv(userID)
		assert.NoError(t, err)
		repoIDs, err := env.RepoIDs(1, 100)
		assert.NoError(t, err)
		assert.Equal(t, expectedRepoIDs, repoIDs)
	}
	testSuccess(2, 1, 100, []int64{3, 5, 32})
	testSuccess(4, 0, 100, []int64{3, 32})
}

func TestAccessibleReposEnv_Repos(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	testSuccess := func(userID int64, expectedRepoIDs []int64) {
		env, err := org.AccessibleReposEnv(userID)
		assert.NoError(t, err)
		repos, err := env.Repos(1, 100)
		assert.NoError(t, err)
		expectedRepos := make([]*Repository, len(expectedRepoIDs))
		for i, repoID := range expectedRepoIDs {
			expectedRepos[i] = db.AssertExistsAndLoadBean(t,
				&Repository{ID: repoID}).(*Repository)
		}
		assert.Equal(t, expectedRepos, repos)
	}
	testSuccess(2, []int64{3, 5, 32})
	testSuccess(4, []int64{3, 32})
}

func TestAccessibleReposEnv_MirrorRepos(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	testSuccess := func(userID int64, expectedRepoIDs []int64) {
		env, err := org.AccessibleReposEnv(userID)
		assert.NoError(t, err)
		repos, err := env.MirrorRepos()
		assert.NoError(t, err)
		expectedRepos := make([]*Repository, len(expectedRepoIDs))
		for i, repoID := range expectedRepoIDs {
			expectedRepos[i] = db.AssertExistsAndLoadBean(t,
				&Repository{ID: repoID}).(*Repository)
		}
		assert.Equal(t, expectedRepos, repos)
	}
	testSuccess(2, []int64{5})
	testSuccess(4, []int64{})
}

func TestHasOrgVisibleTypePublic(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	owner := db.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	user3 := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)

	const newOrgName = "test-org-public"
	org := &User{
		Name:       newOrgName,
		Visibility: structs.VisibleTypePublic,
	}

	db.AssertNotExistsBean(t, &User{Name: org.Name, Type: UserTypeOrganization})
	assert.NoError(t, CreateOrganization(org, owner))
	org = db.AssertExistsAndLoadBean(t,
		&User{Name: org.Name, Type: UserTypeOrganization}).(*User)
	test1 := HasOrgOrUserVisible(org, owner)
	test2 := HasOrgOrUserVisible(org, user3)
	test3 := HasOrgOrUserVisible(org, nil)
	assert.True(t, test1) // owner of org
	assert.True(t, test2) // user not a part of org
	assert.True(t, test3) // logged out user
}

func TestHasOrgVisibleTypeLimited(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	owner := db.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	user3 := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)

	const newOrgName = "test-org-limited"
	org := &User{
		Name:       newOrgName,
		Visibility: structs.VisibleTypeLimited,
	}

	db.AssertNotExistsBean(t, &User{Name: org.Name, Type: UserTypeOrganization})
	assert.NoError(t, CreateOrganization(org, owner))
	org = db.AssertExistsAndLoadBean(t,
		&User{Name: org.Name, Type: UserTypeOrganization}).(*User)
	test1 := HasOrgOrUserVisible(org, owner)
	test2 := HasOrgOrUserVisible(org, user3)
	test3 := HasOrgOrUserVisible(org, nil)
	assert.True(t, test1)  // owner of org
	assert.True(t, test2)  // user not a part of org
	assert.False(t, test3) // logged out user
}

func TestHasOrgVisibleTypePrivate(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	owner := db.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	user3 := db.AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)

	const newOrgName = "test-org-private"
	org := &User{
		Name:       newOrgName,
		Visibility: structs.VisibleTypePrivate,
	}

	db.AssertNotExistsBean(t, &User{Name: org.Name, Type: UserTypeOrganization})
	assert.NoError(t, CreateOrganization(org, owner))
	org = db.AssertExistsAndLoadBean(t,
		&User{Name: org.Name, Type: UserTypeOrganization}).(*User)
	test1 := HasOrgOrUserVisible(org, owner)
	test2 := HasOrgOrUserVisible(org, user3)
	test3 := HasOrgOrUserVisible(org, nil)
	assert.True(t, test1)  // owner of org
	assert.False(t, test2) // user not a part of org
	assert.False(t, test3) // logged out user
}

func TestGetUsersWhoCanCreateOrgRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	users, err := GetUsersWhoCanCreateOrgRepo(3)
	assert.NoError(t, err)
	assert.Len(t, users, 2)
	var ids []int64
	for i := range users {
		ids = append(ids, users[i].ID)
	}
	assert.ElementsMatch(t, ids, []int64{2, 28})

	users, err = GetUsersWhoCanCreateOrgRepo(7)
	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.EqualValues(t, 5, users[0].ID)
}
