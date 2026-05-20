// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"sync"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	auth_source "code.gitea.io/gitea/services/auth/source"

	"github.com/stretchr/testify/assert"
)

func TestSyncGroupsToTeamsConcurrentRuns(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: team.OrgID})

	sourceGroupTeamMapping := map[string]map[string][]string{
		"ldap-group": {
			org.Name: []string{team.Name},
		},
	}

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			assert.NoError(t, auth_source.SyncGroupsToTeams(ctx, user, container.SetOf("ldap-group"), sourceGroupTeamMapping, true))
		})
		wg.Go(func() {
			assert.NoError(t, auth_source.SyncGroupsToTeams(ctx, user, container.Set[string]{}, sourceGroupTeamMapping, true))
		})
	}
	wg.Wait()

	memberCount, err := db.GetEngine(ctx).Count(&organization.TeamUser{OrgID: team.OrgID, TeamID: team.ID, UID: user.ID})
	assert.NoError(t, err)
	assert.LessOrEqual(t, memberCount, int64(1))

	team = unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: team.ID})
	totalCount, err := db.GetEngine(ctx).Count(&organization.TeamUser{OrgID: team.OrgID, TeamID: team.ID})
	assert.NoError(t, err)
	assert.EqualValues(t, team.NumMembers, totalCount)
	assert.GreaterOrEqual(t, team.NumMembers, 0)
}
