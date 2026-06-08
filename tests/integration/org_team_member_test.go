// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	"gitea.dev/models/organization"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/models/usergroup"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrgTeamAddDirectMemberWithExistingGroupMembershipShowsWarning(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	ctx := t.Context()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})
	group := &usergroup.UserGroup{Name: "web-warning-direct-team-member"}
	require.NoError(t, usergroup.CreateUserGroup(ctx, group))
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, group.ID, user.ID))
	require.NoError(t, organization.AddUserGroupToTeam(ctx, team.ID, group.ID, team.OrgID))

	isDirectMember, err := organization.IsTeamMember(ctx, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isDirectMember)

	teamURL := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.Name)
	session := loginUser(t, "user1")
	req := NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"uid":   "1",
		"uname": user.Name,
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	flash := session.GetCookieFlashMessage()
	assert.Empty(t, flash.ErrorMsg)
	assert.Contains(t, flash.WarningMsg, group.Name)

	isDirectMember, err = organization.IsTeamMember(ctx, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isDirectMember)
}
