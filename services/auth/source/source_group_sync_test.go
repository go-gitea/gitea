// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package source

import (
	"testing"

	"gitea.dev/models/organization"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	org_service "gitea.dev/services/org"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{})
}

func TestSyncGroupsToTeams(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("SyncAddRemove", func(t *testing.T) {
		sourceUserGroups := container.SetOf("groupA")
		sourceGroupTeamMapping := map[string]map[string][]string{
			"groupA": {"org3": {"Owners", "team1"}},
			"groupB": {"org3": {"Owners", "Team2"}},
		}

		// Deduplication: "Owners" must not be in the remove list when groupA grants it,
		// while "team2" (only mapped by groupB, which the user is not in) must remain in the remove list.
		membershipsToAdd, membershipsToRemove := resolveMappedMemberships(sourceUserGroups, sourceGroupTeamMapping)
		assert.ElementsMatch(t, []string{"owners", "team1"}, membershipsToAdd["org3"])
		assert.ElementsMatch(t, []string{"team2"}, membershipsToRemove["org3"])
	})

	t.Run("LastOwnerRemovalSkipped", func(t *testing.T) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		org3 := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})

		getUserTeamNames := func(t *testing.T) (ret []string) {
			userTeams, err := organization.GetUserOrgTeams(t.Context(), org3.ID, user2.ID)
			require.NoError(t, err)
			for _, team := range userTeams {
				ret = append(ret, team.Name)
			}
			return ret
		}

		// The last owner should not be removed from the owners team
		// "teamCreateRepo" is always kept because it is not listed in the group mapping
		testSyncUserWithoutGroupMapping := func(t *testing.T) {
			userGroup := container.SetOf("user2Group")
			sourceGroupTeamMapping := map[string]map[string][]string{"otherGroup": {"org3": []string{"Owners", "TEAM1"}}}
			require.NoError(t, SyncGroupsToTeams(t.Context(), user2, userGroup, sourceGroupTeamMapping, true))
		}

		// 1. "user2" is the only owner, so its "owners" team is kept
		assert.ElementsMatch(t, []string{"Owners", "team1", "teamCreateRepo"}, getUserTeamNames(t))
		testSyncUserWithoutGroupMapping(t)
		assert.ElementsMatch(t, []string{"Owners", "teamCreateRepo"}, getUserTeamNames(t))
		// 2. there are other owners, so the user2 is removed from the "owners" team
		teamOwners, _ := organization.GetTeam(t.Context(), org3.ID, "owners")
		_ = org_service.AddTeamMember(t.Context(), teamOwners, user4)
		testSyncUserWithoutGroupMapping(t)
		assert.ElementsMatch(t, []string{"teamCreateRepo"}, getUserTeamNames(t))
	})
}
