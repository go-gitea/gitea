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

func TestOrgUserGroupPageShowsEffectiveMembers(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	ctx := t.Context()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})

	parent := &usergroup.UserGroup{Name: "org-group-parent"}
	require.NoError(t, usergroup.CreateUserGroup(ctx, parent))
	child := &usergroup.UserGroup{Name: "org-group-child", ParentID: parent.ID}
	require.NoError(t, usergroup.CreateUserGroup(ctx, child))
	require.NoError(t, usergroup.AddUserToUserGroup(ctx, child.ID, user.ID))
	require.NoError(t, organization.AddUserGroupToTeam(ctx, team.ID, parent.ID, team.OrgID))

	session := loginUser(t, "user2")
	req := NewRequestf(t, "GET", "/-/user-groups/%d?org=%s", parent.ID, org.Name)
	resp := session.MakeRequest(t, req, http.StatusOK)

	body := resp.Body.String()
	assert.Contains(t, body, parent.Name)
	assert.Contains(t, body, user.Name)
	assert.Contains(t, body, fmt.Sprintf("/org/%s/teams", org.Name))
	assert.NotContains(t, body, fmt.Sprintf("/org/%s/teams/%s", org.Name, team.LowerName))
}
