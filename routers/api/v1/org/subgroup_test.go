// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.dev/models/organization"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/contexttest"
	org_service "gitea.dev/services/org"
	webhook_service "gitea.dev/services/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		SetUp: func() error {
			setting.LoadQueueSettings()
			return webhook_service.Init()
		},
	})
}

func newOrgAPICtx(t *testing.T, reqPath string, doerID int64, orgName string) (*context.APIContext, *httptest.ResponseRecorder) {
	t.Helper()
	ctx, resp := contexttest.MockAPIContext(t, reqPath)
	ctx.Org = new(context.APIOrganization)
	if doerID > 0 {
		ctx.Doer = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: doerID})
		ctx.IsSigned = true
	}
	if orgName != "" {
		o, err := organization.GetOrgByName(ctx, orgName)
		require.NoError(t, err)
		ctx.Org.Organization = o
		ctx.ContextUser = o.AsUser()
	}
	return ctx, resp
}

func TestSubgroupAPIEndpoints(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// 1. POST /api/v1/orgs/org3/subgroups -> create "api-platform" (limited).
	ctx, resp := newOrgAPICtx(t, "POST /api/v1/orgs/org3/subgroups", 2, "org3")
	web.SetForm(ctx, &api.CreateSubgroupOption{
		UserName:    "api-platform",
		FullName:    "API Platform",
		Description: "Platform subgroup",
		Visibility:  api.VisibilityStringLimited,
	})
	CreateSubgroup(ctx)
	require.Equal(t, http.StatusCreated, resp.Code)

	var createdPlatform api.Organization
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &createdPlatform))
	assert.Equal(t, "api-platform", createdPlatform.Name)
	assert.Equal(t, int64(3), createdPlatform.ParentOrgID)
	assert.Equal(t, "org3/api-platform", createdPlatform.FullPath)
	assert.Equal(t, api.VisibilityStringLimited, createdPlatform.Visibility)

	// 2. Reject child with more permissive visibility ("public") under limited "api-platform".
	ctx, resp = newOrgAPICtx(t, "POST /api/v1/orgs/api-platform/subgroups", 2, "api-platform")
	web.SetForm(ctx, &api.CreateSubgroupOption{
		UserName:   "api-public-reject",
		Visibility: api.VisibilityStringPublic,
	})
	CreateSubgroup(ctx)
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)

	// 3. POST /api/v1/orgs with parent_org="api-platform" -> create nested "api-billing" (inherits limited when empty).
	ctx, resp = newOrgAPICtx(t, "POST /api/v1/orgs", 2, "")
	web.SetForm(ctx, &api.CreateOrgOption{
		UserName:  "api-billing",
		FullName:  "Billing Subgroup",
		ParentOrg: "api-platform",
	})
	Create(ctx)
	require.Equal(t, http.StatusCreated, resp.Code)

	var createdBilling api.Organization
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &createdBilling))
	assert.Equal(t, createdPlatform.ID, createdBilling.ParentOrgID)
	assert.Equal(t, "org3/api-platform/api-billing", createdBilling.FullPath)
	assert.Equal(t, api.VisibilityStringLimited, createdBilling.Visibility)

	// 4. GET /api/v1/orgs/org3/subgroups (direct vs recursive=true).
	ctx, resp = newOrgAPICtx(t, "GET /api/v1/orgs/org3/subgroups", 2, "org3")
	ListSubgroups(ctx)
	require.Equal(t, http.StatusOK, resp.Code)
	var directSubs []api.Organization
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &directSubs))
	require.Len(t, directSubs, 1)
	assert.Equal(t, "api-platform", directSubs[0].Name)
	assert.Equal(t, "1", resp.Header().Get("X-Total-Count"))

	ctx, resp = newOrgAPICtx(t, "GET /api/v1/orgs/org3/subgroups?recursive=true", 2, "org3")
	ListSubgroups(ctx)
	require.Equal(t, http.StatusOK, resp.Code)
	var recursiveSubs []api.Organization
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &recursiveSubs))
	require.Len(t, recursiveSubs, 2)
	assert.Equal(t, "api-platform", recursiveSubs[0].Name)
	assert.Equal(t, "api-billing", recursiveSubs[1].Name)
	assert.Equal(t, "2", resp.Header().Get("X-Total-Count"))

	// Anonymous caller cannot see limited subgroups.
	ctx, resp = newOrgAPICtx(t, "GET /api/v1/orgs/org3/subgroups?recursive=true", 0, "org3")
	ListSubgroups(ctx)
	require.Equal(t, http.StatusOK, resp.Code)
	var anonSubs []api.Organization
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &anonSubs))
	assert.Empty(t, anonSubs)

	// 5. GET /api/v1/orgs/api-billing/ancestors returns [org3, api-platform].
	ctx, resp = newOrgAPICtx(t, "GET /api/v1/orgs/api-billing/ancestors", 2, "api-billing")
	ListAncestors(ctx)
	require.Equal(t, http.StatusOK, resp.Code)
	var ancestors []api.Organization
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &ancestors))
	require.Len(t, ancestors, 2)
	assert.Equal(t, "org3", ancestors[0].Name)
	assert.Equal(t, "api-platform", ancestors[1].Name)

	// 5b. Non-admin inherited owner (user2, removed from api-billing's direct membership) satisfies
	// IsOrganizationMemberOrOwner and gets HTTP 200/201 on subgroup team list/create while IsOrganizationMember stays false.
	billingOrg, err := organization.GetOrgByName(t.Context(), "api-billing")
	require.NoError(t, err)
	billingOwnerTeam, err := billingOrg.GetOwnerTeam(t.Context())
	require.NoError(t, err)
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	require.NoError(t, org_service.AddTeamMember(t.Context(), billingOwnerTeam, user4))
	require.NoError(t, org_service.RemoveOrgUser(t.Context(), billingOrg, user2))

	directMem, err := organization.IsOrganizationMember(t.Context(), billingOrg.ID, user2.ID)
	require.NoError(t, err)
	assert.False(t, directMem)
	memOrOwner, err := organization.IsOrganizationMemberOrOwner(t.Context(), billingOrg.ID, user2.ID)
	require.NoError(t, err)
	assert.True(t, memOrOwner)

	ctx, resp = newOrgAPICtx(t, "GET /api/v1/orgs/api-billing/teams", 2, "api-billing")
	ListTeams(ctx)
	require.Equal(t, http.StatusOK, resp.Code)

	ctx, resp = newOrgAPICtx(t, "POST /api/v1/orgs/api-billing/teams", 2, "api-billing")
	web.SetForm(ctx, &api.CreateTeamOption{
		Name:                    "billing-ops",
		Permission:              "write",
		IncludesAllRepositories: true,
		Units:                   []string{"repo.code"},
	})
	CreateTeam(ctx)
	require.Equal(t, http.StatusCreated, resp.Code)

	// 6. PATCH /api/v1/orgs/api-platform/parent rejects cycle ("api-billing"), then detaches to root ("").
	ctx, resp = newOrgAPICtx(t, "PATCH /api/v1/orgs/api-platform/parent", 2, "api-platform")
	web.SetForm(ctx, &api.MoveSubgroupOption{ParentOrg: "api-billing"})
	MoveSubgroup(ctx)
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)

	ctx, resp = newOrgAPICtx(t, "PATCH /api/v1/orgs/api-platform/parent", 2, "api-platform")
	web.SetForm(ctx, &api.MoveSubgroupOption{ParentOrg: ""})
	MoveSubgroup(ctx)
	require.Equal(t, http.StatusOK, resp.Code)
	var detachedPlatform api.Organization
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &detachedPlatform))
	assert.Equal(t, int64(0), detachedPlatform.ParentOrgID)
	assert.Empty(t, detachedPlatform.FullPath)

	// 7. DELETE /api/v1/orgs/api-platform returns 422 while child api-billing exists, then 204 once leaf is deleted.
	ctx, resp = newOrgAPICtx(t, "DELETE /api/v1/orgs/api-platform", 2, "api-platform")
	Delete(ctx)
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)

	ctx, resp = newOrgAPICtx(t, "DELETE /api/v1/orgs/api-billing", 2, "api-billing")
	Delete(ctx)
	require.Equal(t, http.StatusNoContent, resp.Code)

	ctx, resp = newOrgAPICtx(t, "DELETE /api/v1/orgs/api-platform", 2, "api-platform")
	Delete(ctx)
	require.Equal(t, http.StatusNoContent, resp.Code)
}
