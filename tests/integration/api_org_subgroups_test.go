// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/organization"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	org_service "gitea.dev/services/org"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIOrgSubgroupInheritedOwnerTeamsAndMembership(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	ctx := t.Context()

	org3 := unittest.AssertExistsAndLoadBean(t, &organization.Organization{Name: "org3"})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"}) // non-admin owner of org3
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user4"})

	// Create subgroup under org3, make user4 its sole direct owner, and remove user2 from subgroup's direct membership.
	subOrg := &organization.Organization{
		Name:       "org3-sub-teams",
		Visibility: api.VisibleTypeLimited,
		IsActive:   true,
	}
	require.NoError(t, org_service.CreateSubgroup(ctx, org3, subOrg, user2))

	subOwnerTeam, err := subOrg.GetOwnerTeam(ctx)
	require.NoError(t, err)
	require.NoError(t, org_service.AddTeamMember(ctx, subOwnerTeam, user4))
	require.NoError(t, org_service.RemoveOrgUser(ctx, subOrg, user2))

	isDirectMember, err := subOrg.IsOrgMember(ctx, user2.ID)
	require.NoError(t, err)
	assert.False(t, isDirectMember)

	user2Token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
	user5Token := getUserToken(t, "user5", auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

	// 1. Non-admin inherited owner (user2) passes reqOrgMembership on GET /api/v1/orgs/{sub}/teams.
	req := NewRequest(t, "GET", "/api/v1/orgs/org3-sub-teams/teams").AddTokenAuth(user2Token)
	resp := MakeRequest(t, req, http.StatusOK)
	teams := DecodeJSON(t, resp, []*api.Team{})
	assert.Len(t, teams, 1)

	// 2. Non-admin inherited owner (user2) passes reqOrgMembership + reqOrgOwnership on POST /api/v1/orgs/{sub}/teams.
	req = NewRequestWithJSON(t, "POST", "/api/v1/orgs/org3-sub-teams/teams", &api.CreateTeamOption{
		Name:                    "sub-developers",
		Permission:              "write",
		IncludesAllRepositories: true,
		Units:                   []string{"repo.code"},
	}).AddTokenAuth(user2Token)
	resp = MakeRequest(t, req, http.StatusCreated)
	createdTeam := DecodeJSON(t, resp, &api.Team{})
	assert.Equal(t, "sub-developers", createdTeam.Name)

	// 3. Direct membership semantics remain intact: user4 IS a direct member (204), user2 is NOT a direct member (404).
	req = NewRequest(t, "GET", "/api/v1/orgs/org3-sub-teams/members/user4").AddTokenAuth(user2Token)
	MakeRequest(t, req, http.StatusNoContent)

	req = NewRequest(t, "GET", "/api/v1/orgs/org3-sub-teams/members/user2").AddTokenAuth(user2Token)
	MakeRequest(t, req, http.StatusNotFound)

	// 4. Unrelated user5 is still rejected with 403 Forbidden on /teams.
	req = NewRequest(t, "GET", "/api/v1/orgs/org3-sub-teams/teams").AddTokenAuth(user5Token)
	MakeRequest(t, req, http.StatusForbidden)

	// 5. Non-admin inherited owner (user2) can transfer a repository into a private subgroup where user2 is not a direct member.
	privSub := &organization.Organization{
		Name:       "org3-priv-transfer-target",
		Visibility: api.VisibleTypePrivate,
		IsActive:   true,
	}
	require.NoError(t, org_service.CreateSubgroup(ctx, org3, privSub, user2))
	privOwnerTeam, err := privSub.GetOwnerTeam(ctx)
	require.NoError(t, err)
	require.NoError(t, org_service.AddTeamMember(ctx, privOwnerTeam, user4))
	require.NoError(t, org_service.RemoveOrgUser(ctx, privSub, user2))
	assert.False(t, privSub.HasMemberWithUserID(ctx, user2.ID))
	assert.True(t, privSub.HasMemberOrOwnerWithUserID(ctx, user2.ID))

	createRepoReq := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{
		Name:     "transfer-into-priv-subgroup",
		AutoInit: true,
	}).AddTokenAuth(user2Token)
	MakeRequest(t, createRepoReq, http.StatusCreated)

	// Unrelated user5 gets 404 Not Found trying to target the private subgroup.
	createUser5RepoReq := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{
		Name:     "user5-blocked-transfer",
		AutoInit: true,
	}).AddTokenAuth(user5Token)
	MakeRequest(t, createUser5RepoReq, http.StatusCreated)
	badTransferReq := NewRequestWithJSON(t, "POST", "/api/v1/repos/user5/user5-blocked-transfer/transfer", &api.TransferRepoOption{
		NewOwner: privSub.Name,
	}).AddTokenAuth(user5Token)
	MakeRequest(t, badTransferReq, http.StatusNotFound)

	// Inherited owner user2 succeeds (202 Accepted immediate transfer).
	transferReq := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/transfer-into-priv-subgroup/transfer", &api.TransferRepoOption{
		NewOwner: privSub.Name,
	}).AddTokenAuth(user2Token)
	transferResp := MakeRequest(t, transferReq, http.StatusAccepted)
	transferredRepo := DecodeJSON(t, transferResp, &api.Repository{})
	assert.Equal(t, privSub.Name, transferredRepo.Owner.UserName)
}
