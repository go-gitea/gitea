// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package source

import (
	"testing"

	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{})
}

func TestSyncGroupsToTeams_LastOwnerNotRemoved(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Fixtures: in org3, user2 is the only member of the "Owners" team, and
	// team "team1" (a non-owner team) has user2 and user4 as members.
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org3 := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	ownersTeam, err := org3.GetOwnerTeam(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, ownersTeam.NumMembers)

	sourceUserGroups := container.SetOf("groupA")
	sourceGroupTeamMapping := map[string]map[string][]string{
		"groupA": {"org3": {"Owners"}},
		"groupB": {"org3": {"Owners", "team1"}},
	}

	// Deduplication: "Owners" must not be in the remove list when groupA grants it,
	// while "team1" (only mapped by groupB, which the user is not in) must remain.
	membershipsToAdd, membershipsToRemove := resolveMappedMemberships(sourceUserGroups, sourceGroupTeamMapping)
	assert.Contains(t, membershipsToAdd["org3"], "Owners")
	assert.NotContains(t, membershipsToRemove["org3"], "Owners")
	assert.Contains(t, membershipsToRemove["org3"], "team1")

	// End-to-end: sync must not fail with ErrLastOrgOwner.
	require.NoError(t, SyncGroupsToTeams(t.Context(), user2, sourceUserGroups, sourceGroupTeamMapping, true))

	// User2 must still be a member of the Owners team.
	stillOwner, err := organization.IsTeamMember(t.Context(), org3.ID, ownersTeam.ID, user2.ID)
	require.NoError(t, err)
	assert.True(t, stillOwner)
}

func TestSyncGroupsToTeams_LastOwnerRemovalSkippedWhenNotGranted(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org3 := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	ownersTeam, err := org3.GetOwnerTeam(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, ownersTeam.NumMembers)

	// User is in no mapped group, so Owners is scheduled for removal with no
	// offsetting add. The remove must be attempted and gracefully skipped.
	sourceUserGroups := container.SetOf[string]()
	sourceGroupTeamMapping := map[string]map[string][]string{
		"groupA": {"org3": {"Owners"}},
	}

	require.NoError(t, SyncGroupsToTeams(t.Context(), user2, sourceUserGroups, sourceGroupTeamMapping, true))

	stillOwner, err := organization.IsTeamMember(t.Context(), org3.ID, ownersTeam.ID, user2.ID)
	require.NoError(t, err)
	assert.True(t, stillOwner, "sole owner must remain after sync that couldn't remove them")
}

func TestResolveMappedMemberships_Dedup(t *testing.T) {
	sourceUserGroups := container.SetOf("in1", "in2")
	sourceGroupTeamMapping := map[string]map[string][]string{
		"in1":  {"orgA": {"Owners"}, "orgB": {"team1"}},
		"in2":  {"orgB": {"team1"}},                     // duplicate add, no effect on remove
		"out1": {"orgA": {"Owners"}, "orgB": {"team2"}}, // orgA fully overlaps add; orgB team2 has no add
		"out2": {"orgC": {"team3"}},                     // no add for orgC
	}

	membershipsToAdd, membershipsToRemove := resolveMappedMemberships(sourceUserGroups, sourceGroupTeamMapping)

	assert.ElementsMatch(t, []string{"Owners"}, membershipsToAdd["orgA"])
	assert.ElementsMatch(t, []string{"team1", "team1"}, membershipsToAdd["orgB"])
	_, hasOrgA := membershipsToRemove["orgA"]
	assert.False(t, hasOrgA, "orgA must be deleted from remove map when all removals overlap adds")
	assert.ElementsMatch(t, []string{"team2"}, membershipsToRemove["orgB"])
	assert.ElementsMatch(t, []string{"team3"}, membershipsToRemove["orgC"])
}
