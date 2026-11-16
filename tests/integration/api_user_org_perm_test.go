// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	auth_model "gitea.dev/models/auth"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestPermissionsAPI(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	MakeRequest(t, NewRequest(t, "GET", "/api/v1/users/user1/orgs/org6/permissions"), http.StatusUnauthorized)

	userToken := func(doer string, scopes ...auth_model.AccessTokenScope) string {
		return getUserToken(t, doer, append(scopes, auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadUser)...)
	}
	requestPermissions := func(t *testing.T, token, username, orgName string, expectedStatus int) *httptest.ResponseRecorder {
		return MakeRequest(t, NewRequestf(t, "GET", "/api/v1/users/%s/orgs/%s/permissions", username, orgName).AddTokenAuth(token), expectedStatus)
	}

	adminToken := userToken("user1", auth_model.AccessTokenScopeAll)
	ownerToken := userToken("user2")
	outsiderToken := userToken("user8")
	assert.Equal(t, "user not found", DecodeJSON(t, requestPermissions(t, adminToken, "unknown", "org25", http.StatusNotFound), &api.APIError{}).Message)
	requestPermissions(t, adminToken, "user1", "unknown", http.StatusNotFound)
	requestPermissions(t, outsiderToken, "user5", "privated_org", http.StatusNotFound)

	for _, testCase := range []struct {
		name, token, username, orgName string
		expected                       api.OrganizationPermissions
	}{
		{"Owner", ownerToken, "user2", "org3", api.OrganizationPermissions{IsOwner: true, IsAdmin: true, CanWrite: true, CanRead: true, CanCreateRepository: true}},
		{"SelfConcealed", userToken("user4"), "user4", "org3", api.OrganizationPermissions{CanWrite: true, CanRead: true}},
		{"PublicOnlyTokenSelf", userToken("user2", auth_model.AccessTokenScopePublicOnly), "user2", "org3", api.OrganizationPermissions{}},
		{"MemberSeesConcealed", ownerToken, "user4", "org3", api.OrganizationPermissions{CanWrite: true, CanRead: true}},
		{"OutsiderConcealed", outsiderToken, "user4", "org3", api.OrganizationPermissions{}},
		{"OutsiderPublic", outsiderToken, "user28", "org3", api.OrganizationPermissions{}},
		{"RestrictedMemberConcealedTeammate", userToken("user29"), "user20", "org17", api.OrganizationPermissions{}},
		{"SiteAdmin", adminToken, "user28", "org3", api.OrganizationPermissions{IsAdmin: true, CanWrite: true, CanRead: true, CanCreateRepository: true}},
		{"SiteAdminCanNotCreateRepo", adminToken, "user28", "org6", api.OrganizationPermissions{IsAdmin: true, CanWrite: true, CanRead: true}},
		{"SiteAdminCanRead", adminToken, "user24", "org25", api.OrganizationPermissions{CanRead: true}},
		{"SiteAdminPrivateOrgConcealed", adminToken, "user5", "privated_org", api.OrganizationPermissions{CanWrite: true, CanRead: true, CanCreateRepository: true}},
		{"SiteAdminScopedTokenConcealed", userToken("user1"), "user5", "privated_org", api.OrganizationPermissions{}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, *DecodeJSON(t, requestPermissions(t, testCase.token, testCase.username, testCase.orgName, http.StatusOK), &api.OrganizationPermissions{}))
		})
	}
}
