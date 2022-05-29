// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package organization

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
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
		org := unittest.AssertExistsAndLoadBean(t, &Organization{ID: testCase.OrgID}).(*Organization)
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
		org := unittest.AssertExistsAndLoadBean(t, &Organization{ID: testCase.OrgID}).(*Organization)
		isMember, err := org.IsOrgMember(testCase.UserID)
		assert.NoError(t, err)
		assert.Equal(t, testCase.ExpectedMember, isMember)
	}
}

func TestUser_GetTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &Organization{ID: 3}).(*Organization)
	team, err := org.GetTeam("team1")
	assert.NoError(t, err)
	assert.Equal(t, org.ID, team.OrgID)
	assert.Equal(t, "team1", team.LowerName)

	_, err = org.GetTeam("does not exist")
	assert.True(t, IsErrTeamNotExist(err))

	nonOrg := unittest.AssertExistsAndLoadBean(t, &Organization{ID: 2}).(*Organization)
	_, err = nonOrg.GetTeam("team")
	assert.True(t, IsErrTeamNotExist(err))
}

func TestUser_GetOwnerTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &Organization{ID: 3}).(*Organization)
	team, err := org.GetOwnerTeam()
	assert.NoError(t, err)
	assert.Equal(t, org.ID, team.OrgID)

	nonOrg := unittest.AssertExistsAndLoadBean(t, &Organization{ID: 2}).(*Organization)
	_, err = nonOrg.GetOwnerTeam()
	assert.True(t, IsErrTeamNotExist(err))
}

func TestUser_GetTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &Organization{ID: 3}).(*Organization)
	teams, err := org.LoadTeams()
	assert.NoError(t, err)
	if assert.Len(t, teams, 4) {
		assert.Equal(t, int64(1), teams[0].ID)
		assert.Equal(t, int64(2), teams[1].ID)
		assert.Equal(t, int64(12), teams[2].ID)
		assert.Equal(t, int64(7), teams[3].ID)
	}
}

func TestUser_GetMembers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &Organization{ID: 3}).(*Organization)
	members, _, err := org.GetMembers()
	assert.NoError(t, err)
	if assert.Len(t, members, 3) {
		assert.Equal(t, int64(2), members[0].ID)
		assert.Equal(t, int64(28), members[1].ID)
		assert.Equal(t, int64(4), members[2].ID)
	}
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
	expected, err := db.GetEngine(db.DefaultContext).Where("type=?", user_model.UserTypeOrganization).Count(&Organization{})
	assert.NoError(t, err)
	cnt, err := CountOrgs(FindOrgOptions{IncludePrivate: true})
	assert.NoError(t, err)
	assert.Equal(t, expected, cnt)
}

func TestIsOrganizationOwner(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(orgID, userID int64, expected bool) {
		isOwner, err := IsOrganizationOwner(db.DefaultContext, orgID, userID)
		assert.NoError(t, err)
		assert.EqualValues(t, expected, isOwner)
	}
	test(3, 2, true)
	test(3, 3, false)
	test(6, 5, true)
	test(6, 4, false)
	test(unittest.NonexistentID, unittest.NonexistentID, false)
}

func TestIsOrganizationMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(orgID, userID int64, expected bool) {
		isMember, err := IsOrganizationMember(db.DefaultContext, orgID, userID)
		assert.NoError(t, err)
		assert.EqualValues(t, expected, isMember)
	}
	test(3, 2, true)
	test(3, 3, false)
	test(3, 4, true)
	test(6, 5, true)
	test(6, 4, false)
	test(unittest.NonexistentID, unittest.NonexistentID, false)
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
	test(unittest.NonexistentID, unittest.NonexistentID, false)
}

func TestFindOrgs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	orgs, err := FindOrgs(FindOrgOptions{
		UserID:         4,
		IncludePrivate: true,
	})
	assert.NoError(t, err)
	if assert.Len(t, orgs, 1) {
		assert.EqualValues(t, 3, orgs[0].ID)
	}

	orgs, err = FindOrgs(FindOrgOptions{
		UserID:         4,
		IncludePrivate: false,
	})
	assert.NoError(t, err)
	assert.Len(t, orgs, 0)

	total, err := CountOrgs(FindOrgOptions{
		UserID:         4,
		IncludePrivate: true,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, total)
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

	orgUsers, err := GetOrgUsersByOrgID(db.DefaultContext, &FindOrgMembersOpts{
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

	orgUsers, err = GetOrgUsersByOrgID(db.DefaultContext, &FindOrgMembersOpts{
		ListOptions: db.ListOptions{},
		OrgID:       unittest.NonexistentID,
		PublicOnly:  false,
	})
	assert.NoError(t, err)
	assert.Len(t, orgUsers, 0)
}

func TestChangeOrgUserStatus(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(orgID, userID int64, public bool) {
		assert.NoError(t, ChangeOrgUserStatus(orgID, userID, public))
		orgUser := unittest.AssertExistsAndLoadBean(t, &OrgUser{OrgID: orgID, UID: userID}).(*OrgUser)
		assert.Equal(t, public, orgUser.IsPublic)
	}

	testSuccess(3, 2, false)
	testSuccess(3, 2, false)
	testSuccess(3, 4, true)
	assert.NoError(t, ChangeOrgUserStatus(unittest.NonexistentID, unittest.NonexistentID, true))
}

func TestUser_GetUserTeamIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &Organization{ID: 3}).(*Organization)
	testSuccess := func(userID int64, expected []int64) {
		teamIDs, err := org.GetUserTeamIDs(userID)
		assert.NoError(t, err)
		assert.Equal(t, expected, teamIDs)
	}
	testSuccess(2, []int64{1, 2})
	testSuccess(4, []int64{2})
	testSuccess(unittest.NonexistentID, []int64{})
}

func TestAccessibleReposEnv_CountRepos(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &Organization{ID: 3}).(*Organization)
	testSuccess := func(userID, expectedCount int64) {
		env, err := AccessibleReposEnv(db.DefaultContext, org, userID)
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
	org := unittest.AssertExistsAndLoadBean(t, &Organization{ID: 3}).(*Organization)
	testSuccess := func(userID, _, pageSize int64, expectedRepoIDs []int64) {
		env, err := AccessibleReposEnv(db.DefaultContext, org, userID)
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
	org := unittest.AssertExistsAndLoadBean(t, &Organization{ID: 3}).(*Organization)
	testSuccess := func(userID int64, expectedRepoIDs []int64) {
		env, err := AccessibleReposEnv(db.DefaultContext, org, userID)
		assert.NoError(t, err)
		repos, err := env.Repos(1, 100)
		assert.NoError(t, err)
		expectedRepos := make([]*repo_model.Repository, len(expectedRepoIDs))
		for i, repoID := range expectedRepoIDs {
			expectedRepos[i] = unittest.AssertExistsAndLoadBean(t,
				&repo_model.Repository{ID: repoID}).(*repo_model.Repository)
		}
		assert.Equal(t, expectedRepos, repos)
	}
	testSuccess(2, []int64{3, 5, 32})
	testSuccess(4, []int64{3, 32})
}

func TestAccessibleReposEnv_MirrorRepos(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &Organization{ID: 3}).(*Organization)
	testSuccess := func(userID int64, expectedRepoIDs []int64) {
		env, err := AccessibleReposEnv(db.DefaultContext, org, userID)
		assert.NoError(t, err)
		repos, err := env.MirrorRepos()
		assert.NoError(t, err)
		expectedRepos := make([]*repo_model.Repository, len(expectedRepoIDs))
		for i, repoID := range expectedRepoIDs {
			expectedRepos[i] = unittest.AssertExistsAndLoadBean(t,
				&repo_model.Repository{ID: repoID}).(*repo_model.Repository)
		}
		assert.Equal(t, expectedRepos, repos)
	}
	testSuccess(2, []int64{5})
	testSuccess(4, []int64{})
}

func TestHasOrgVisibleTypePublic(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	user3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3}).(*user_model.User)

	const newOrgName = "test-org-public"
	org := &Organization{
		Name:       newOrgName,
		Visibility: structs.VisibleTypePublic,
	}

	unittest.AssertNotExistsBean(t, &user_model.User{Name: org.Name, Type: user_model.UserTypeOrganization})
	assert.NoError(t, CreateOrganization(org, owner))
	org = unittest.AssertExistsAndLoadBean(t,
		&Organization{Name: org.Name, Type: user_model.UserTypeOrganization}).(*Organization)
	test1 := HasOrgOrUserVisible(db.DefaultContext, org.AsUser(), owner)
	test2 := HasOrgOrUserVisible(db.DefaultContext, org.AsUser(), user3)
	test3 := HasOrgOrUserVisible(db.DefaultContext, org.AsUser(), nil)
	assert.True(t, test1) // owner of org
	assert.True(t, test2) // user not a part of org
	assert.True(t, test3) // logged out user
}

func TestHasOrgVisibleTypeLimited(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	user3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3}).(*user_model.User)

	const newOrgName = "test-org-limited"
	org := &Organization{
		Name:       newOrgName,
		Visibility: structs.VisibleTypeLimited,
	}

	unittest.AssertNotExistsBean(t, &user_model.User{Name: org.Name, Type: user_model.UserTypeOrganization})
	assert.NoError(t, CreateOrganization(org, owner))
	org = unittest.AssertExistsAndLoadBean(t,
		&Organization{Name: org.Name, Type: user_model.UserTypeOrganization}).(*Organization)
	test1 := HasOrgOrUserVisible(db.DefaultContext, org.AsUser(), owner)
	test2 := HasOrgOrUserVisible(db.DefaultContext, org.AsUser(), user3)
	test3 := HasOrgOrUserVisible(db.DefaultContext, org.AsUser(), nil)
	assert.True(t, test1)  // owner of org
	assert.True(t, test2)  // user not a part of org
	assert.False(t, test3) // logged out user
}

func TestHasOrgVisibleTypePrivate(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	user3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3}).(*user_model.User)

	const newOrgName = "test-org-private"
	org := &Organization{
		Name:       newOrgName,
		Visibility: structs.VisibleTypePrivate,
	}

	unittest.AssertNotExistsBean(t, &user_model.User{Name: org.Name, Type: user_model.UserTypeOrganization})
	assert.NoError(t, CreateOrganization(org, owner))
	org = unittest.AssertExistsAndLoadBean(t,
		&Organization{Name: org.Name, Type: user_model.UserTypeOrganization}).(*Organization)
	test1 := HasOrgOrUserVisible(db.DefaultContext, org.AsUser(), owner)
	test2 := HasOrgOrUserVisible(db.DefaultContext, org.AsUser(), user3)
	test3 := HasOrgOrUserVisible(db.DefaultContext, org.AsUser(), nil)
	assert.True(t, test1)  // owner of org
	assert.False(t, test2) // user not a part of org
	assert.False(t, test3) // logged out user
}

func TestGetUsersWhoCanCreateOrgRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	users, err := GetUsersWhoCanCreateOrgRepo(db.DefaultContext, 3)
	assert.NoError(t, err)
	assert.Len(t, users, 2)
	var ids []int64
	for i := range users {
		ids = append(ids, users[i].ID)
	}
	assert.ElementsMatch(t, ids, []int64{2, 28})

	users, err = GetUsersWhoCanCreateOrgRepo(db.DefaultContext, 7)
	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.EqualValues(t, 5, users[0].ID)
}
