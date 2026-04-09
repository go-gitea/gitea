// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package source

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{})
}

// Fixture: team 1 = org3's "Owners" team, num_members=1, sole member is user2.
func TestSyncGroupsToTeams_LastOwnerNotRemoved(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	sourceUserGroups := container.SetOf("groupA")
	sourceGroupTeamMapping := map[string]map[string][]string{
		"groupA": {"org3": {"Owners"}},
		"groupB": {"org3": {"Owners", "team1"}},
	}

	// Verify deduplication: Owners must not be in remove list when groupA grants it.
	membershipsToAdd, membershipsToRemove := resolveMappedMemberships(sourceUserGroups, sourceGroupTeamMapping)
	assert.Contains(t, membershipsToAdd["org3"], "Owners")
	assert.NotContains(t, membershipsToRemove["org3"], "Owners")
	assert.Contains(t, membershipsToRemove["org3"], "team1")

	// End-to-end: sync must not fail with ErrLastOrgOwner.
	err := SyncGroupsToTeams(t.Context(), user2, sourceUserGroups, sourceGroupTeamMapping, true)
	require.NoError(t, err)
}
