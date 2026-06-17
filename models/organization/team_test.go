// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestTeam_IsOwnerTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	assert.True(t, team.IsOwnerTeam())

	team = unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	assert.False(t, team.IsOwnerTeam())
}

func TestTeam_IsMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	assert.True(t, team.IsMember(t.Context(), 2))
	assert.False(t, team.IsMember(t.Context(), 4))
	assert.False(t, team.IsMember(t.Context(), unittest.NonexistentID))

	team = unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	assert.True(t, team.IsMember(t.Context(), 2))
	assert.True(t, team.IsMember(t.Context(), 4))
	assert.False(t, team.IsMember(t.Context(), unittest.NonexistentID))
}

func TestTeam_CanNonMemberReadMeta(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})     // public org
	org35 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 35})   // private org
	member := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})   // member of org 3 and org 35
	outsider := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}) // member of neither org

	test := func(name string, team *organization.Team, org, doer *user_model.User, expected bool) {
		t.Run(name, func(t *testing.T) {
			ok, err := team.CanNonMemberReadMeta(t.Context(), org, doer)
			assert.NoError(t, err)
			assert.Equal(t, expected, ok)
		})
	}

	// Public team is gated only by the parent org's visibility.
	publicTeam := &organization.Team{OrgID: 3, Visibility: structs.VisibleTypePublic}
	test("public team, public org, member", publicTeam, org3, member, true)
	test("public team, public org, outsider", publicTeam, org3, outsider, true)

	// Public team inside a private org: only org members may see it.
	publicTeamPrivOrg := &organization.Team{OrgID: 35, Visibility: structs.VisibleTypePublic}
	test("public team, private org, org member", publicTeamPrivOrg, org35, member, true)
	test("public team, private org, outsider", publicTeamPrivOrg, org35, outsider, false)

	// Limited team: any org member, but never outsiders.
	limitedTeam := &organization.Team{OrgID: 3, Visibility: structs.VisibleTypeLimited}
	test("limited team, org member", limitedTeam, org3, member, true)
	test("limited team, outsider", limitedTeam, org3, outsider, false)

	// Private team is never visible to non-members; members/owners are admitted by the caller.
	privateTeam := &organization.Team{OrgID: 3, Visibility: structs.VisibleTypePrivate}
	test("private team, org member", privateTeam, org3, member, false)
	test("private team, outsider", privateTeam, org3, outsider, false)
}

func TestTeam_GetRepositories(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		repos, err := repo_model.GetTeamRepositories(t.Context(), &repo_model.SearchTeamRepoOptions{
			TeamID: team.ID,
		})
		assert.NoError(t, err)
		assert.Len(t, repos, team.NumRepos)
		for _, repo := range repos {
			unittest.AssertExistsAndLoadBean(t, &organization.TeamRepo{TeamID: teamID, RepoID: repo.ID})
		}
	}
	test(1)
	test(3)
}

func TestTeam_GetMembers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		assert.NoError(t, team.LoadMembers(t.Context()))
		assert.Len(t, team.Members, team.NumMembers)
		for _, member := range team.Members {
			unittest.AssertExistsAndLoadBean(t, &organization.TeamUser{UID: member.ID, TeamID: teamID})
		}
	}
	test(1)
	test(3)
}

func TestGetTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(orgID int64, name string) {
		team, err := organization.GetTeam(t.Context(), orgID, name)
		assert.NoError(t, err)
		assert.Equal(t, orgID, team.OrgID)
		assert.Equal(t, name, team.Name)
	}
	testSuccess(3, "Owners")
	testSuccess(3, "team1")

	_, err := organization.GetTeam(t.Context(), 3, "nonexistent")
	assert.Error(t, err)
	_, err = organization.GetTeam(t.Context(), unittest.NonexistentID, "Owners")
	assert.Error(t, err)
}

func TestGetTeamByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(teamID int64) {
		team, err := organization.GetTeamByID(t.Context(), teamID)
		assert.NoError(t, err)
		assert.Equal(t, teamID, team.ID)
	}
	testSuccess(1)
	testSuccess(2)
	testSuccess(3)
	testSuccess(4)

	_, err := organization.GetTeamByID(t.Context(), unittest.NonexistentID)
	assert.Error(t, err)
}

func TestIsTeamMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(orgID, teamID, userID int64, expected bool) {
		isMember, err := organization.IsTeamMember(t.Context(), orgID, teamID, userID)
		assert.NoError(t, err)
		assert.Equal(t, expected, isMember)
	}

	test(3, 1, 2, true)
	test(3, 1, 4, false)
	test(3, 1, unittest.NonexistentID, false)

	test(3, 2, 2, true)
	test(3, 2, 4, true)

	test(3, unittest.NonexistentID, unittest.NonexistentID, false)
	test(unittest.NonexistentID, unittest.NonexistentID, unittest.NonexistentID, false)
}

func TestGetTeamMembers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		members, err := organization.GetTeamMembers(t.Context(), &organization.SearchMembersOptions{
			TeamID: teamID,
		})
		assert.NoError(t, err)
		assert.Len(t, members, team.NumMembers)
		for _, member := range members {
			unittest.AssertExistsAndLoadBean(t, &organization.TeamUser{UID: member.ID, TeamID: teamID})
		}
	}
	test(1)
	test(3)
}

func TestGetUserTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(userID int64) {
		teams, _, err := organization.SearchTeam(t.Context(), &organization.SearchTeamOptions{UserID: userID})
		assert.NoError(t, err)
		for _, team := range teams {
			unittest.AssertExistsAndLoadBean(t, &organization.TeamUser{TeamID: team.ID, UID: userID})
		}
	}
	test(2)
	test(5)
	test(unittest.NonexistentID)
}

func TestGetUserOrgTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(orgID, userID int64) {
		teams, err := organization.GetUserOrgTeams(t.Context(), orgID, userID)
		assert.NoError(t, err)
		for _, team := range teams {
			assert.Equal(t, orgID, team.OrgID)
			unittest.AssertExistsAndLoadBean(t, &organization.TeamUser{TeamID: team.ID, UID: userID})
		}
	}
	test(3, 2)
	test(3, 4)
	test(3, unittest.NonexistentID)
}

func TestSearchTeamIncludeVisible(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const orgID int64 = 3
	// User 5 is an org member but only belongs to team 1 (Owners) — make sure
	// they don't see team 2 (default private) but do see a freshly added
	// limited team they are not a member of.
	visible := &organization.Team{
		OrgID:      orgID,
		LowerName:  "visible-team",
		Name:       "visible-team",
		AccessMode: 1, // read
		Visibility: structs.VisibleTypeLimited,
	}
	assert.NoError(t, db.Insert(t.Context(), visible))
	teams, _, err := organization.SearchTeam(t.Context(), &organization.SearchTeamOptions{
		OrgID:               orgID,
		UserID:              2,
		IncludeVisibilities: organization.VisibleTeamVisibilitiesFor(true, true),
	})
	assert.NoError(t, err)
	ids := make(map[int64]bool, len(teams))
	for _, team := range teams {
		assert.Equal(t, orgID, team.OrgID)
		ids[team.ID] = true
	}
	// user 2 is in team 1 and team 2 in org 3, plus should see the new visible team.
	assert.True(t, ids[1], "expected to see team 1 (member)")
	assert.True(t, ids[2], "expected to see team 2 (member)")
	assert.True(t, ids[visible.ID], "expected to see visible team")

	// user 5 is only an org member in team 1, must not see secret team 2 but must see the visible one.
	teams, _, err = organization.SearchTeam(t.Context(), &organization.SearchTeamOptions{
		OrgID:               orgID,
		UserID:              5,
		IncludeVisibilities: organization.VisibleTeamVisibilitiesFor(true, true),
	})
	assert.NoError(t, err)
	ids = make(map[int64]bool, len(teams))
	for _, team := range teams {
		ids[team.ID] = true
	}
	assert.False(t, ids[2], "user 5 must not see private team 2")
	assert.True(t, ids[visible.ID], "user 5 must see the limited team")
}

func TestHasTeamRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID, repoID int64, expected bool) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		assert.Equal(t, expected, organization.HasTeamRepo(t.Context(), team.OrgID, teamID, repoID))
	}
	test(1, 1, false)
	test(1, 3, true)
	test(1, 5, true)
	test(1, unittest.NonexistentID, false)

	test(2, 3, true)
	test(2, 5, false)
}

func TestUsersInTeamsCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamIDs, userIDs []int64, expected int64) {
		count, err := organization.UsersInTeamsCount(t.Context(), teamIDs, userIDs)
		assert.NoError(t, err)
		assert.Equal(t, expected, count)
	}

	test([]int64{2}, []int64{1, 2, 3, 4}, 1)          // only userid 2
	test([]int64{1, 2, 3, 4, 5}, []int64{2, 5}, 2)    // userid 2,4
	test([]int64{1, 2, 3, 4, 5}, []int64{2, 3, 5}, 3) // userid 2,4,5
}

func TestIsUsableTeamName(t *testing.T) {
	assert.NoError(t, organization.IsUsableTeamName("usable"))
	assert.True(t, db.IsErrNameReserved(organization.IsUsableTeamName("new")))
}
