// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/models/usergroup"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTestGroup is a helper that creates a UserGroup and fails the test immediately
// on any error.
func makeTestGroup(t *testing.T, name string, parentID int64) *usergroup.UserGroup {
	t.Helper()
	g := &usergroup.UserGroup{Name: name, ParentID: parentID}
	require.NoError(t, usergroup.CreateUserGroup(t.Context(), g))
	return g
}

// TestRecalculateUserGroupTeamAccessesAncestorExpansion is the regression
// test for the bug where adding a user to a child group did not update the
// access table for teams that assigned a parent/ancestor group.
//
// Scenario:
//
//	Team T (org 3) assigns group G_root.
//	User is added to G_child (a grandchild of G_root).
//	After RecalculateUserGroupTeamAccesses(G_child),
//	the access table must contain an entry for that user on repo 3.
func TestRecalculateUserGroupTeamAccessesAncestorExpansion(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Fixtures: team 2, org 3, private repo 3 (via team_repo entry).
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	// user 8 is not a direct member of team 2.
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})

	gRoot := makeTestGroup(t, "rca-root", 0)
	gMid := makeTestGroup(t, "rca-mid", gRoot.ID)
	gChild := makeTestGroup(t, "rca-child", gMid.ID)

	// Assign the root group to the team.
	require.NoError(t, organization.AddUserGroupToTeam(ctx, team.ID, gRoot.ID, team.OrgID))

	// Add user to the grandchild group and trigger recalculation on that group.
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, gChild.ID, user.ID))
	require.NoError(t, RecalculateUserGroupTeamAccesses(ctx, gChild.ID))

	// The access table must now have an entry for the user on this repo.
	require.NoError(t, repo.LoadOwner(ctx))
	entry := &access_model.Access{UserID: user.ID, RepoID: repo.ID}
	has, err := db.GetEngine(ctx).Get(entry)
	require.NoError(t, err)
	assert.True(t, has, "RecalculateUserGroupTeamAccesses must update access for ancestor-assigned teams")
}

// TestAddTeamUserGroupSyncsOrgUser verifies that assigning a user group to
// a team also writes org_user entries for every group member.
func TestAddTeamUserGroupSyncsOrgUser(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})

	g := makeTestGroup(t, "add-sync-g", 0)
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, g.ID, user.ID))

	// Before assignment the user should not be an org member.
	isBefore, err := organization.IsOrganizationMember(ctx, team.OrgID, user.ID)
	require.NoError(t, err)
	require.False(t, isBefore, "user must not be an org member before group is assigned")

	require.NoError(t, AddTeamUserGroup(ctx, team, g.ID))

	isAfter, err := organization.IsOrganizationMember(ctx, team.OrgID, user.ID)
	require.NoError(t, err)
	assert.True(t, isAfter, "user must become an org member after group is assigned to team")
}

// TestRemoveTeamUserGroupCleansOrgUser checks that when a user group is
// removed from a team, members who have no other access path are removed from
// org_user.
func TestRemoveTeamUserGroupCleansOrgUser(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})

	g := makeTestGroup(t, "rm-sync-g", 0)
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, g.ID, user.ID))
	require.NoError(t, AddTeamUserGroup(ctx, team, g.ID))

	// Confirm membership was created.
	isMember, err := organization.IsOrganizationMember(ctx, team.OrgID, user.ID)
	require.NoError(t, err)
	require.True(t, isMember, "precondition: user must be org member after group assignment")

	require.NoError(t, RemoveTeamUserGroup(ctx, team, g.ID))

	isGone, err := organization.IsOrganizationMember(ctx, team.OrgID, user.ID)
	require.NoError(t, err)
	assert.False(t, isGone, "user must be removed from org when the only group granting access is removed")
}

// TestDoublePathOrgUserRetained verifies that when a user has both a direct
// team membership and a user group membership, removing the group does NOT
// remove the user from org_user.
func TestDoublePathOrgUserRetained(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// user 4 is a direct member of team 2 (see fixtures/team_user.yml).
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	// Confirm user is already a direct team member.
	isDirect, err := organization.IsTeamMember(ctx, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	require.True(t, isDirect, "precondition: user 4 must be a direct team 2 member")

	g := makeTestGroup(t, "double-path-g", 0)
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, g.ID, user.ID))
	require.NoError(t, AddTeamUserGroup(ctx, team, g.ID))

	// Remove the group — direct membership still exists.
	require.NoError(t, RemoveTeamUserGroup(ctx, team, g.ID))

	stillMember, err := organization.IsOrganizationMember(ctx, team.OrgID, user.ID)
	require.NoError(t, err)
	assert.True(t, stillMember, "user must remain org member because direct team membership still exists")
}

// TestSyncGroupMemberToOrgsAddRemove exercises the single-user sync helper
// used by UserGroupAddMember / UserGroupRemoveMember.
func TestSyncGroupMemberToOrgsAddRemove(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})

	g := makeTestGroup(t, "sync-add-rm-g", 0)
	// First assign the group to the team, then sync for the user.
	require.NoError(t, organization.AddUserGroupToTeam(ctx, team.ID, g.ID, team.OrgID))
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, g.ID, user.ID))

	// Simulate UserGroupAddMember path.
	require.NoError(t, SyncGroupMemberToOrgs(ctx, g.ID, user.ID, true))

	inOrg, err := organization.IsOrganizationMember(ctx, team.OrgID, user.ID)
	require.NoError(t, err)
	assert.True(t, inOrg, "SyncGroupMemberToOrgs(add) must add user to org")

	// Now simulate UserGroupRemoveMember path.
	require.NoError(t, usergroup.RemoveUserFromUserGroup(ctx, g.ID, user.ID))
	require.NoError(t, SyncGroupMemberToOrgs(ctx, g.ID, user.ID, false))

	notInOrg, err := organization.IsOrganizationMember(ctx, team.OrgID, user.ID)
	require.NoError(t, err)
	assert.False(t, notInOrg, "SyncGroupMemberToOrgs(remove) must remove user from org when no other path exists")
}

// TestSyncReplaceUserGroupMembersSyncsAncestorAssignedOrgs verifies that
// replacing members on a child group also updates org_user for teams that assign
// an ancestor group.
func TestSyncReplaceUserGroupMembersSyncsAncestorAssignedOrgs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 24})
	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 35})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})

	parent := makeTestGroup(t, "replace-parent-g", 0)
	child := makeTestGroup(t, "replace-child-g", parent.ID)
	require.NoError(t, AddTeamUserGroup(ctx, team, parent.ID))

	inOrgBefore, err := organization.IsOrganizationMember(ctx, org.ID, user.ID)
	require.NoError(t, err)
	require.False(t, inOrgBefore)

	require.NoError(t, SyncReplaceUserGroupMembers(ctx, child.ID, []int64{user.ID}))

	inOrgAfter, err := organization.IsOrganizationMember(ctx, org.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, inOrgAfter, "replacing child-group members must sync org_user for ancestor-assigned teams")
}

// TestUpdateUserGroupWithSyncParentChangeRemovesAccess verifies that changing
// the parent hierarchy immediately revokes repo/org access that depended on the
// old ancestor chain.
func TestUpdateUserGroupWithSyncParentChangeRemovesAccess(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})

	parent := makeTestGroup(t, "move-parent-g", 0)
	child := makeTestGroup(t, "move-child-g", parent.ID)
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, child.ID, user.ID))
	require.NoError(t, AddTeamUserGroup(ctx, team, parent.ID))

	entry := &access_model.Access{UserID: user.ID, RepoID: repo.ID}
	hasBefore, err := db.GetEngine(ctx).Get(entry)
	require.NoError(t, err)
	require.True(t, hasBefore)

	inOrgBefore, err := organization.IsOrganizationMember(ctx, team.OrgID, user.ID)
	require.NoError(t, err)
	require.True(t, inOrgBefore)

	child.ParentID = 0
	require.NoError(t, UpdateUserGroupWithSync(ctx, child))

	hasAfter, err := db.GetEngine(ctx).Get(entry)
	require.NoError(t, err)
	assert.False(t, hasAfter, "changing the group parent must revoke repo access from the old ancestor assignment")

	inOrgAfter, err := organization.IsOrganizationMember(ctx, team.OrgID, user.ID)
	require.NoError(t, err)
	assert.False(t, inOrgAfter, "changing the group parent must revoke org membership from the old ancestor assignment")
}

// TestSearchRepositoryVisibleToUserGroupMember is the end-to-end regression
// test for the org repo-list bug: a user group member could not see the team's
// private repo in the organisation repository list.
//
// The test calls SearchRepository with the user as Actor and OwnerID=org,
// exactly as the org home page handler does, and asserts the private repo
// appears in the results.
func TestSearchRepositoryVisibleToUserGroupMember(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Fixtures: team 2 (org 3, authorize=write) owns repo 3 (private).
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	require.True(t, repo.IsPrivate)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})

	g := makeTestGroup(t, "search-vis-g", 0)
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, g.ID, user.ID))

	// AddTeamUserGroup: assigns group to team AND recalculates access AND
	// updates org_user — this is the full production code path.
	require.NoError(t, AddTeamUserGroup(ctx, team, g.ID))

	// Verify the access table was populated (prerequisite).
	entry := &access_model.Access{UserID: user.ID, RepoID: repo.ID}
	has, err := db.GetEngine(ctx).Get(entry)
	require.NoError(t, err)
	require.True(t, has, "access table must have entry after AddTeamUserGroup")

	// Now simulate SearchRepository as called from the org home page.
	repos, count, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{PageSize: 50, Page: 1},
		OwnerID:     team.OrgID,
		Private:     true, // user is signed in
		Actor:       user,
	})
	require.NoError(t, err)

	found := false
	for _, r := range repos {
		if r.ID == repo.ID {
			found = true
		}
	}
	assert.True(t, found,
		"private repo must appear in org repo list for user group member (count=%d)", count)
}

// TestSearchRepositoryVisibleViaUserGroupWithoutAccessRow verifies that repo
// search still finds private repos through user group team membership even if
// the access table entry is stale or missing.
func TestSearchRepositoryVisibleViaUserGroupWithoutAccessRow(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})
	require.True(t, repo.IsPrivate)

	g := makeTestGroup(t, "search-cond-g", 0)
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, g.ID, user.ID))
	require.NoError(t, AddTeamUserGroup(ctx, team, g.ID))

	_, err := db.GetEngine(ctx).Where("user_id = ? AND repo_id = ?", user.ID, repo.ID).Delete(new(access_model.Access))
	require.NoError(t, err)

	repos, count, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{PageSize: 50, Page: 1},
		OwnerID:     team.OrgID,
		Private:     true,
		Actor:       user,
	})
	require.NoError(t, err)

	found := false
	for _, r := range repos {
		if r.ID == repo.ID {
			found = true
		}
	}
	assert.True(t, found,
		"private repo must appear in org repo list via SearchRepositoryCondition even without an access row (count=%d)", count)
}

// TestSearchRepositoryVisibleAfterUserAddedToGroup tests the reverse order:
// the group is assigned to the team first (empty group), then the user is added
// to the group (as done via UserGroupAddMember in the web handler).
func TestSearchRepositoryVisibleAfterUserAddedToGroup(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	require.True(t, repo.IsPrivate)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})

	g := makeTestGroup(t, "late-add-g", 0)

	// Step 1: assign the (empty) group to the team first.
	require.NoError(t, AddTeamUserGroup(ctx, team, g.ID))

	// Access entry must NOT exist yet (group was empty when assigned).
	entryBefore := &access_model.Access{UserID: user.ID, RepoID: repo.ID}
	hasBefore, err := db.GetEngine(ctx).Get(entryBefore)
	require.NoError(t, err)
	assert.False(t, hasBefore, "no access entry expected before user is added to group")

	// Step 2: add user to the group (mirrors UserGroupAddMember in the web handler).
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, g.ID, user.ID))
	require.NoError(t, RecalculateUserGroupTeamAccesses(ctx, g.ID))
	require.NoError(t, SyncGroupMemberToOrgs(ctx, g.ID, user.ID, true))

	// Access entry must now exist.
	entryAfter := &access_model.Access{UserID: user.ID, RepoID: repo.ID}
	hasAfter, err := db.GetEngine(ctx).Get(entryAfter)
	require.NoError(t, err)
	require.True(t, hasAfter, "access entry must exist after user is added to group")

	// SearchRepository (org home page path) must return the private repo.
	repos, count, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{PageSize: 50, Page: 1},
		OwnerID:     team.OrgID,
		Private:     true,
		Actor:       user,
	})
	require.NoError(t, err)

	found := false
	for _, r := range repos {
		if r.ID == repo.ID {
			found = true
		}
	}
	assert.True(t, found,
		"private repo must appear after user is added to already-assigned group (count=%d)", count)
}

// TestPrivateOrgUserGroupMemberCanAccessPage verifies that a user who gains
// team access via a user group in a PRIVATE organisation is also added to
// org_user, so the org.Visibility==Private check in OrgAssignment doesn't 404
// them before they ever reach the repository list.
func TestPrivateOrgUserGroupMemberCanAccessPage(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// org 35 (visibility=2 = private), team 24 (write, includes_all_repositories=true).
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 24})
	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 35})
	// user 8 is not a member of this private org.
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})

	// Confirm: user is not in org_user before the group is set up.
	isBefore, err := organization.IsOrganizationMember(ctx, org.ID, user.ID)
	require.NoError(t, err)
	require.False(t, isBefore, "precondition: user must not be in private org before test")

	g := makeTestGroup(t, "priv-org-g", 0)
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, g.ID, user.ID))

	// Assigning the group to the team must add the user to org_user.
	require.NoError(t, AddTeamUserGroup(ctx, team, g.ID))

	isAfter, err := organization.IsOrganizationMember(ctx, org.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isAfter,
		"user group member must be added to org_user for a private org so the org page is accessible")
}

// TestPrivateOrgUserGroupMemberSeesPrivateRepo is the full end-to-end test
// for a PRIVATE organisation: a user group member must be able to see the
// team's private repo in the org's repository list (SearchRepository).
//
// This covers the regression: on private orgs, users not in org_user get 404
// before SearchRepository even runs (org.Visibility==Private → RequireMember).
// And even when they can reach the page, they need an access-table entry.
func TestPrivateOrgUserGroupMemberSeesPrivateRepo(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// org 23 (private), team 17 (write), repo 41 (private, owned by org 23).
	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 23})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 17})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 41})
	require.True(t, repo.IsPrivate)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})

	// Explicitly add the private repo to the team (team 17 starts with 0 repos).
	require.NoError(t, organization.AddTeamRepo(ctx, org.ID, team.ID, repo.ID))
	require.NoError(t, repo.LoadOwner(ctx))
	require.NoError(t, access_model.RecalculateTeamAccesses(ctx, repo, 0))

	// Confirm user is not yet an org member.
	isBefore, err := organization.IsOrganizationMember(ctx, org.ID, user.ID)
	require.NoError(t, err)
	require.False(t, isBefore)

	g := makeTestGroup(t, "priv-repo-g", 0)
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, g.ID, user.ID))
	require.NoError(t, AddTeamUserGroup(ctx, team, g.ID))

	// User must now be an org member (so RequireMember check passes).
	isOrgMember, err := organization.IsOrganizationMember(ctx, org.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isOrgMember, "user must be in org_user for the private org")

	// SearchRepository (org home page) must return the private repo.
	repos, count, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{PageSize: 50, Page: 1},
		OwnerID:     org.ID,
		Private:     true,
		Actor:       user,
	})
	require.NoError(t, err)

	found := false
	for _, r := range repos {
		if r.ID == repo.ID {
			found = true
		}
	}
	assert.True(t, found,
		"private repo in a private org must appear in org repo list for user group member (count=%d)", count)
}

// TestReAddUserGroupToIncludeAllRepositoriesTeam restores access for a team
// that includes all org repos even if legacy data has no team_repo rows.
func TestReAddUserGroupToIncludeAllRepositoriesTeam(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 23})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 17})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 41})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})
	require.True(t, repo.IsPrivate)

	team.IncludesAllRepositories = true
	_, err := db.GetEngine(ctx).ID(team.ID).Cols("includes_all_repositories").Update(team)
	require.NoError(t, err)
	require.NoError(t, organization.RemoveTeamRepo(ctx, team.ID, repo.ID))

	g := makeTestGroup(t, "include-all-readd-g", 0)
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, g.ID, user.ID))
	require.NoError(t, AddTeamUserGroup(ctx, team, g.ID))
	require.NoError(t, RemoveTeamUserGroup(ctx, team, g.ID))
	require.NoError(t, AddTeamUserGroup(ctx, team, g.ID))

	repos, count, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{PageSize: 50, Page: 1},
		OwnerID:     org.ID,
		Private:     true,
		Actor:       user,
	})
	require.NoError(t, err)

	found := false
	for _, r := range repos {
		if r.ID == repo.ID {
			found = true
		}
	}
	assert.True(t, found,
		"private repo in include-all team must reappear after removing and re-adding the user group (count=%d)", count)
}
