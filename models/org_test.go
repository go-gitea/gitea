// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUser_IsOwnedBy(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	assert.True(t, org.IsOwnedBy(2))
	assert.False(t, org.IsOwnedBy(1))
	assert.False(t, org.IsOwnedBy(3))
	assert.False(t, org.IsOwnedBy(4))

	nonOrg := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	assert.False(t, nonOrg.IsOwnedBy(2))
	assert.False(t, nonOrg.IsOwnedBy(3))
}

func TestUser_IsOrgMember(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	assert.True(t, org.IsOrgMember(2))
	assert.True(t, org.IsOrgMember(4))
	assert.False(t, org.IsOrgMember(1))
	assert.False(t, org.IsOrgMember(3))

	nonOrg := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	assert.False(t, nonOrg.IsOrgMember(2))
	assert.False(t, nonOrg.IsOrgMember(3))
}

func TestUser_GetTeam(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	team, err := org.GetTeam("team1")
	assert.NoError(t, err)
	assert.Equal(t, org.ID, team.OrgID)
	assert.Equal(t, "team1", team.LowerName)

	_, err = org.GetTeam("does not exist")
	assert.Equal(t, ErrTeamNotExist, err)

	nonOrg := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	_, err = nonOrg.GetTeam("team")
	assert.Equal(t, ErrTeamNotExist, err)
}

func TestUser_GetOwnerTeam(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	team, err := org.GetOwnerTeam()
	assert.NoError(t, err)
	assert.Equal(t, org.ID, team.OrgID)

	nonOrg := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	_, err = nonOrg.GetOwnerTeam()
	assert.Equal(t, ErrTeamNotExist, err)
}

func TestUser_GetTeams(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	assert.NoError(t, org.GetTeams())
	if assert.Len(t, org.Teams, 2) {
		assert.Equal(t, int64(1), org.Teams[0].ID)
		assert.Equal(t, int64(2), org.Teams[1].ID)
	}
}

func TestUser_GetMembers(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	assert.NoError(t, org.GetMembers())
	if assert.Len(t, org.Members, 2) {
		assert.Equal(t, int64(2), org.Members[0].ID)
		assert.Equal(t, int64(4), org.Members[1].ID)
	}
}

func TestUser_AddMember(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)

	// add a user that is not a member
	AssertNotExistsBean(t, &OrgUser{UID: 5, OrgID: 3})
	prevNumMembers := org.NumMembers
	assert.NoError(t, org.AddMember(5))
	AssertExistsAndLoadBean(t, &OrgUser{UID: 5, OrgID: 3})
	org = AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	assert.Equal(t, prevNumMembers+1, org.NumMembers)

	// add a user that is already a member
	AssertExistsAndLoadBean(t, &OrgUser{UID: 4, OrgID: 3})
	prevNumMembers = org.NumMembers
	assert.NoError(t, org.AddMember(4))
	AssertExistsAndLoadBean(t, &OrgUser{UID: 4, OrgID: 3})
	org = AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	assert.Equal(t, prevNumMembers, org.NumMembers)

	CheckConsistencyFor(t, &User{})
}

func TestUser_RemoveMember(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)

	// remove a user that is a member
	AssertExistsAndLoadBean(t, &OrgUser{UID: 4, OrgID: 3})
	prevNumMembers := org.NumMembers
	assert.NoError(t, org.RemoveMember(4))
	AssertNotExistsBean(t, &OrgUser{UID: 4, OrgID: 3})
	org = AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	assert.Equal(t, prevNumMembers-1, org.NumMembers)

	// remove a user that is not a member
	AssertNotExistsBean(t, &OrgUser{UID: 5, OrgID: 3})
	prevNumMembers = org.NumMembers
	assert.NoError(t, org.RemoveMember(5))
	AssertNotExistsBean(t, &OrgUser{UID: 5, OrgID: 3})
	org = AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	assert.Equal(t, prevNumMembers, org.NumMembers)

	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestUser_RemoveOrgRepo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{OwnerID: org.ID}).(*Repository)

	// remove a repo that does belong to org
	AssertExistsAndLoadBean(t, &TeamRepo{RepoID: repo.ID, OrgID: org.ID})
	assert.NoError(t, org.RemoveOrgRepo(repo.ID))
	AssertNotExistsBean(t, &TeamRepo{RepoID: repo.ID, OrgID: org.ID})
	AssertExistsAndLoadBean(t, &Repository{ID: repo.ID}) // repo should still exist

	// remove a repo that does not belong to org
	assert.NoError(t, org.RemoveOrgRepo(repo.ID))
	AssertNotExistsBean(t, &TeamRepo{RepoID: repo.ID, OrgID: org.ID})

	assert.NoError(t, org.RemoveOrgRepo(NonexistentID))

	CheckConsistencyFor(t,
		&User{ID: org.ID},
		&Team{OrgID: org.ID},
		&Repository{ID: repo.ID})
}

func TestCreateOrganization(t *testing.T) {
	// successful creation of org
	assert.NoError(t, PrepareTestDatabase())

	owner := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	const newOrgName = "neworg"
	org := &User{
		Name: newOrgName,
	}

	AssertNotExistsBean(t, &User{Name: newOrgName, Type: UserTypeOrganization})
	assert.NoError(t, CreateOrganization(org, owner))
	org = AssertExistsAndLoadBean(t,
		&User{Name: newOrgName, Type: UserTypeOrganization}).(*User)
	ownerTeam := AssertExistsAndLoadBean(t,
		&Team{Name: ownerTeamName, OrgID: org.ID}).(*Team)
	AssertExistsAndLoadBean(t, &TeamUser{UID: owner.ID, TeamID: ownerTeam.ID})
	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestCreateOrganization2(t *testing.T) {
	// unauthorized creation of org
	assert.NoError(t, PrepareTestDatabase())

	owner := AssertExistsAndLoadBean(t, &User{ID: 5}).(*User)
	const newOrgName = "neworg"
	org := &User{
		Name: newOrgName,
	}

	AssertNotExistsBean(t, &User{Name: newOrgName, Type: UserTypeOrganization})
	err := CreateOrganization(org, owner)
	assert.Error(t, err)
	assert.True(t, IsErrUserNotAllowedCreateOrg(err))
	AssertNotExistsBean(t, &User{Name: newOrgName, Type: UserTypeOrganization})
	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestCreateOrganization3(t *testing.T) {
	// create org with same name as existent org
	assert.NoError(t, PrepareTestDatabase())

	owner := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	org := &User{Name: "user3"}                       // should already exist
	AssertExistsAndLoadBean(t, &User{Name: org.Name}) // sanity check
	err := CreateOrganization(org, owner)
	assert.Error(t, err)
	assert.True(t, IsErrUserAlreadyExist(err))
	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestCreateOrganization4(t *testing.T) {
	// create org with unusable name
	assert.NoError(t, PrepareTestDatabase())

	owner := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	err := CreateOrganization(&User{Name: "assets"}, owner)
	assert.Error(t, err)
	assert.True(t, IsErrNameReserved(err))
	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestGetOrgByName(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	org, err := GetOrgByName("user3")
	assert.NoError(t, err)
	assert.EqualValues(t, 3, org.ID)
	assert.Equal(t, "user3", org.Name)

	org, err = GetOrgByName("user2") // user2 is an individual
	assert.True(t, IsErrOrgNotExist(err))

	org, err = GetOrgByName("") // corner case
	assert.True(t, IsErrOrgNotExist(err))
}

func TestCountOrganizations(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	expected, err := x.Where("type=?", UserTypeOrganization).Count(&User{})
	assert.NoError(t, err)
	assert.Equal(t, expected, CountOrganizations())
}

func TestOrganizations(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	testSuccess := func(opts *SearchUserOptions, expectedOrgIDs []int64) {
		orgs, err := Organizations(opts)
		assert.NoError(t, err)
		if assert.Len(t, orgs, len(expectedOrgIDs)) {
			for i, expectedOrgID := range expectedOrgIDs {
				assert.EqualValues(t, expectedOrgID, orgs[i].ID)
			}
		}
	}
	testSuccess(&SearchUserOptions{OrderBy: "id ASC", Page: 1, PageSize: 2},
		[]int64{3, 6})

	testSuccess(&SearchUserOptions{OrderBy: "id ASC", Page: 2, PageSize: 2},
		[]int64{7})

	testSuccess(&SearchUserOptions{Page: 3, PageSize: 2},
		[]int64{})
}

func TestDeleteOrganization(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 6}).(*User)
	assert.NoError(t, DeleteOrganization(org))
	AssertNotExistsBean(t, &User{ID: 6})
	AssertNotExistsBean(t, &OrgUser{OrgID: 6})
	AssertNotExistsBean(t, &Team{OrgID: 6})

	org = AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	err := DeleteOrganization(org)
	assert.Error(t, err)
	assert.True(t, IsErrUserOwnRepos(err))

	nonOrg := AssertExistsAndLoadBean(t, &User{ID: 5}).(*User)
	assert.Error(t, DeleteOrganization(nonOrg))
	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestIsOrganizationOwner(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	assert.True(t, IsOrganizationOwner(3, 2))
	assert.False(t, IsOrganizationOwner(3, 3))
	assert.True(t, IsOrganizationOwner(6, 5))
	assert.False(t, IsOrganizationOwner(6, 4))
	assert.False(t, IsOrganizationOwner(NonexistentID, NonexistentID))
}

func TestIsOrganizationMember(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	assert.True(t, IsOrganizationMember(3, 2))
	assert.False(t, IsOrganizationMember(3, 3))
	assert.True(t, IsOrganizationMember(3, 4))
	assert.True(t, IsOrganizationMember(6, 5))
	assert.False(t, IsOrganizationMember(6, 4))
	assert.False(t, IsOrganizationMember(NonexistentID, NonexistentID))
}

func TestIsPublicMembership(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	assert.True(t, IsPublicMembership(3, 2))
	assert.False(t, IsPublicMembership(3, 3))
	assert.False(t, IsPublicMembership(3, 4))
	assert.True(t, IsPublicMembership(6, 5))
	assert.False(t, IsPublicMembership(6, 4))
	assert.False(t, IsPublicMembership(NonexistentID, NonexistentID))
}

func TestGetOrgsByUserID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

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
	assert.NoError(t, PrepareTestDatabase())

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
	assert.NoError(t, PrepareTestDatabase())

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
	assert.NoError(t, PrepareTestDatabase())

	orgUsers, err := GetOrgUsersByUserID(5, true)
	assert.NoError(t, err)
	if assert.Len(t, orgUsers, 2) {
		assert.Equal(t, OrgUser{
			ID:       orgUsers[0].ID,
			OrgID:    6,
			UID:      5,
			IsOwner:  true,
			IsPublic: true,
			NumTeams: 1}, *orgUsers[0])
		assert.Equal(t, OrgUser{
			ID:       orgUsers[1].ID,
			OrgID:    7,
			UID:      5,
			IsOwner:  true,
			IsPublic: false,
			NumTeams: 1}, *orgUsers[1])
	}

	publicOrgUsers, err := GetOrgUsersByUserID(5, false)
	assert.NoError(t, err)
	assert.Len(t, publicOrgUsers, 1)
	assert.Equal(t, *orgUsers[0], *publicOrgUsers[0])

	orgUsers, err = GetOrgUsersByUserID(1, true)
	assert.NoError(t, err)
	assert.Len(t, orgUsers, 0)
}

func TestGetOrgUsersByOrgID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	orgUsers, err := GetOrgUsersByOrgID(3)
	assert.NoError(t, err)
	if assert.Len(t, orgUsers, 2) {
		assert.Equal(t, OrgUser{
			ID:       orgUsers[0].ID,
			OrgID:    3,
			UID:      2,
			IsOwner:  true,
			IsPublic: true,
			NumTeams: 1}, *orgUsers[0])
		assert.Equal(t, OrgUser{
			ID:       orgUsers[1].ID,
			OrgID:    3,
			UID:      4,
			IsOwner:  false,
			IsPublic: false,
			NumTeams: 0}, *orgUsers[1])
	}

	orgUsers, err = GetOrgUsersByOrgID(NonexistentID)
	assert.NoError(t, err)
	assert.Len(t, orgUsers, 0)
}

func TestChangeOrgUserStatus(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testSuccess := func(orgID, userID int64, public bool) {
		assert.NoError(t, ChangeOrgUserStatus(orgID, userID, public))
		orgUser := AssertExistsAndLoadBean(t, &OrgUser{OrgID: orgID, UID: userID}).(*OrgUser)
		assert.Equal(t, public, orgUser.IsPublic)
	}

	testSuccess(3, 2, false)
	testSuccess(3, 2, false)
	testSuccess(3, 4, true)
	assert.NoError(t, ChangeOrgUserStatus(NonexistentID, NonexistentID, true))
}

func TestAddOrgUser(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	testSuccess := func(orgID, userID int64) {
		org := AssertExistsAndLoadBean(t, &User{ID: orgID}).(*User)
		expectedNumMembers := org.NumMembers
		if !BeanExists(t, &OrgUser{OrgID: orgID, UID: userID}) {
			expectedNumMembers++
		}
		assert.NoError(t, AddOrgUser(orgID, userID))
		AssertExistsAndLoadBean(t, &OrgUser{OrgID: orgID, UID: userID})
		org = AssertExistsAndLoadBean(t, &User{ID: orgID}).(*User)
		assert.EqualValues(t, expectedNumMembers, org.NumMembers)
	}
	testSuccess(3, 5)
	testSuccess(3, 5)
	testSuccess(6, 2)
	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestRemoveOrgUser(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	testSuccess := func(orgID, userID int64) {
		org := AssertExistsAndLoadBean(t, &User{ID: orgID}).(*User)
		expectedNumMembers := org.NumMembers
		if BeanExists(t, &OrgUser{OrgID: orgID, UID: userID}) {
			expectedNumMembers--
		}
		assert.NoError(t, RemoveOrgUser(orgID, userID))
		AssertNotExistsBean(t, &OrgUser{OrgID: orgID, UID: userID})
		org = AssertExistsAndLoadBean(t, &User{ID: orgID}).(*User)
		assert.EqualValues(t, expectedNumMembers, org.NumMembers)
	}
	testSuccess(3, 4)
	testSuccess(3, 4)

	err := RemoveOrgUser(7, 5)
	assert.Error(t, err)
	assert.True(t, IsErrLastOrgOwner(err))
	AssertExistsAndLoadBean(t, &OrgUser{OrgID: 7, UID: 5})
	CheckConsistencyFor(t, &User{}, &Team{})
}

func TestUser_GetUserTeamIDs(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	testSuccess := func(userID int64, expected []int64) {
		teamIDs, err := org.GetUserTeamIDs(userID)
		assert.NoError(t, err)
		assert.Equal(t, expected, teamIDs)
	}
	testSuccess(2, []int64{1, 2})
	testSuccess(4, []int64{2})
	testSuccess(NonexistentID, []int64{})
}

func TestAccessibleReposEnv_CountRepos(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	testSuccess := func(userID, expectedCount int64) {
		env, err := org.AccessibleReposEnv(userID)
		assert.NoError(t, err)
		count, err := env.CountRepos()
		assert.NoError(t, err)
		assert.EqualValues(t, expectedCount, count)
	}
	testSuccess(2, 2)
	testSuccess(4, 1)
}

func TestAccessibleReposEnv_RepoIDs(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	testSuccess := func(userID, page, pageSize int64, expectedRepoIDs []int64) {
		env, err := org.AccessibleReposEnv(userID)
		assert.NoError(t, err)
		repoIDs, err := env.RepoIDs(1, 100)
		assert.NoError(t, err)
		assert.Equal(t, expectedRepoIDs, repoIDs)
	}
	testSuccess(2, 1, 100, []int64{3, 5})
	testSuccess(4, 0, 100, []int64{3})
}

func TestAccessibleReposEnv_Repos(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	testSuccess := func(userID int64, expectedRepoIDs []int64) {
		env, err := org.AccessibleReposEnv(userID)
		assert.NoError(t, err)
		repos, err := env.Repos(1, 100)
		assert.NoError(t, err)
		expectedRepos := make([]*Repository, len(expectedRepoIDs))
		for i, repoID := range expectedRepoIDs {
			expectedRepos[i] = AssertExistsAndLoadBean(t,
				&Repository{ID: repoID}).(*Repository)
		}
		assert.Equal(t, expectedRepos, repos)
	}
	testSuccess(2, []int64{3, 5})
	testSuccess(4, []int64{3})
}

func TestAccessibleReposEnv_MirrorRepos(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	testSuccess := func(userID int64, expectedRepoIDs []int64) {
		env, err := org.AccessibleReposEnv(userID)
		assert.NoError(t, err)
		repos, err := env.MirrorRepos()
		assert.NoError(t, err)
		expectedRepos := make([]*Repository, len(expectedRepoIDs))
		for i, repoID := range expectedRepoIDs {
			expectedRepos[i] = AssertExistsAndLoadBean(t,
				&Repository{ID: repoID}).(*Repository)
		}
		assert.Equal(t, expectedRepos, repos)
	}
	testSuccess(2, []int64{5})
	testSuccess(4, []int64{})
}
