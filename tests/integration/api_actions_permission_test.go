// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testActionUserSignIn(t *testing.T) {
	req := NewRequest(t, "GET", "/api/v1/user").
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusOK)

	u := DecodeJSON(t, resp, &api.User{})
	assert.Equal(t, "gitea-actions", u.UserName)
}

func testActionUserAccessPublicRepo(t *testing.T) {
	req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/raw/README.md").
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "file", resp.Header().Get("x-gitea-object-type"))

	defer test.MockVariableValue(&setting.Service.RequireSignInViewStrict, true)()

	req = NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/raw/README.md").
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "file", resp.Header().Get("x-gitea-object-type"))
}

func testActionUserNoAccessOtherPrivateRepo(t *testing.T) {
	req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo2/raw/README.md").
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	MakeRequest(t, req, http.StatusNotFound)
}

func TestActionUserAccessPermission(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("ActionUserSignIn", testActionUserSignIn)
	t.Run("ActionUserAccessPublicRepo", testActionUserAccessPublicRepo)
	t.Run("ActionUserNoAccessOtherPrivateRepo", testActionUserNoAccessOtherPrivateRepo)
}

func TestAPIActionsTokenPermissionsSettings(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository)
	maxPerms := &api.ActionsTokenPermissions{
		Code: "read", Issues: "write", PullRequests: "none", Packages: "read",
		Actions: "none", Wiki: "none", Releases: "read", Projects: "none",
	}

	req := NewRequest(t, "GET", "/api/v1/orgs/org3/actions/permissions").AddTokenAuth(token)
	orgPerms := DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.OrgActionsPermissions{})
	assert.Equal(t, api.OrgActionsPermissions{TokenPermissionMode: "permissive", AllowedCrossRepos: []string{}}, *orgPerms)

	req = NewRequestWithJSON(t, "PUT", "/api/v1/orgs/org3/actions/permissions", &api.EditOrgActionsPermissionsOption{
		TokenPermissionMode: "restricted",
		MaxTokenPermissions: maxPerms,
		AllowedCrossRepos:   []string{"repo3", "repo3"},
	}).AddTokenAuth(token)
	orgPerms = DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.OrgActionsPermissions{})
	assert.Equal(t, api.OrgActionsPermissions{TokenPermissionMode: "restricted", MaxTokenPermissions: maxPerms, AllowedCrossRepos: []string{"repo3"}}, *orgPerms)

	req = NewRequestWithJSON(t, "PUT", "/api/v1/orgs/org3/actions/permissions", &api.EditOrgActionsPermissionsOption{
		TokenPermissionMode: "restricted",
		AllowedCrossRepos:   []string{"no-such-repo"},
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	invalidPerms := *maxPerms
	invalidPerms.Code = "admin"
	for _, perms := range []*api.ActionsTokenPermissions{&invalidPerms, {Code: "read"}} {
		req = NewRequestWithJSON(t, "PUT", "/api/v1/orgs/org3/actions/permissions", &api.EditOrgActionsPermissionsOption{
			TokenPermissionMode: "restricted",
			MaxTokenPermissions: perms,
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusUnprocessableEntity)
	}

	user4Token := getUserToken(t, "user4", auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadRepository)
	req = NewRequest(t, "GET", "/api/v1/orgs/org3/actions/permissions").AddTokenAuth(user4Token)
	MakeRequest(t, req, http.StatusForbidden)
	req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/actions/permissions").AddTokenAuth(user4Token)
	MakeRequest(t, req, http.StatusForbidden)
	req = NewRequest(t, "GET", "/api/v1/repos/user2/repo2/actions/permissions").AddTokenAuth(token)
	apiErr := DecodeJSON(t, MakeRequest(t, req, http.StatusNotFound), &api.APIError{})
	assert.Equal(t, "actions unit is not enabled", apiErr.Message)

	// the repository follows its owner until it overrides the owner config
	req = NewRequest(t, "GET", "/api/v1/repos/org3/repo3/actions/permissions").AddTokenAuth(token)
	repoPerms := DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.RepoActionsPermissions{})
	assert.Equal(t, api.RepoActionsPermissions{TokenPermissionMode: "restricted", MaxTokenPermissions: maxPerms}, *repoPerms)

	req = NewRequestWithJSON(t, "PUT", "/api/v1/repos/org3/repo3/actions/permissions", &api.EditRepoActionsPermissionsOption{
		OverrideOwnerConfig: true,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	req = NewRequestWithJSON(t, "PUT", "/api/v1/repos/org3/repo3/actions/permissions", &api.EditRepoActionsPermissionsOption{
		OverrideOwnerConfig: true,
		TokenPermissionMode: "permissive",
	}).AddTokenAuth(token)
	repoPerms = DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.RepoActionsPermissions{})
	assert.Equal(t, api.RepoActionsPermissions{OverrideOwnerConfig: true, TokenPermissionMode: "permissive"}, *repoPerms)

	req = NewRequestWithJSON(t, "PUT", "/api/v1/repos/org3/repo3/actions/permissions", &api.EditRepoActionsPermissionsOption{}).AddTokenAuth(token)
	repoPerms = DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.RepoActionsPermissions{})
	assert.Equal(t, api.RepoActionsPermissions{TokenPermissionMode: "restricted", MaxTokenPermissions: maxPerms}, *repoPerms)
	actionsUnit, err := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}).GetUnit(t.Context(), unit.TypeActions)
	require.NoError(t, err)
	assert.Equal(t, repo_model.ActionsTokenPermissionModePermissive, actionsUnit.ActionsConfig().TokenPermissionMode)
}
