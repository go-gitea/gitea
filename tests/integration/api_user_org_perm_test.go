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

	requestPermissions := func(t *testing.T, doer, username, orgName string, expectedStatus int) *httptest.ResponseRecorder {
		token := getUserToken(t, doer, auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadUser)
		req := NewRequestf(t, "GET", "/api/v1/users/%s/orgs/%s/permissions", username, orgName).AddTokenAuth(token)
		return MakeRequest(t, req, expectedStatus)
	}

	resp := requestPermissions(t, "user1", "unknown", "org25", http.StatusNotFound)
	assert.Equal(t, "user redirect does not exist [name: unknown]", DecodeJSON(t, resp, &api.APIError{}).Message)
	requestPermissions(t, "user1", "user1", "unknown", http.StatusNotFound)
	requestPermissions(t, "user8", "user5", "privated_org", http.StatusNotFound)

	for _, testCase := range []struct {
		name, doer, username, orgName string
		expected                      api.OrganizationPermissions
	}{
		{"Owner", "user2", "user2", "org3", api.OrganizationPermissions{IsOwner: true, IsAdmin: true, CanWrite: true, CanRead: true, CanCreateRepository: true}},
		{"SelfConcealed", "user4", "user4", "org3", api.OrganizationPermissions{CanWrite: true, CanRead: true}},
		{"MemberSeesConcealed", "user2", "user4", "org3", api.OrganizationPermissions{CanWrite: true, CanRead: true}},
		{"OutsiderConcealed", "user8", "user4", "org3", api.OrganizationPermissions{}},
		{"OutsiderPublic", "user8", "user28", "org3", api.OrganizationPermissions{}},
		{"RestrictedMemberConcealedNonTeammate", "user29", "user18", "org17", api.OrganizationPermissions{}},
		{"SiteAdmin", "user1", "user28", "org3", api.OrganizationPermissions{IsAdmin: true, CanWrite: true, CanRead: true, CanCreateRepository: true}},
		{"SiteAdminCanNotCreateRepo", "user1", "user28", "org6", api.OrganizationPermissions{IsAdmin: true, CanWrite: true, CanRead: true}},
		{"SiteAdminCanRead", "user1", "user24", "org25", api.OrganizationPermissions{CanRead: true}},
		{"SiteAdminPrivateOrgConcealed", "user1", "user5", "privated_org", api.OrganizationPermissions{CanWrite: true, CanRead: true, CanCreateRepository: true}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resp := requestPermissions(t, testCase.doer, testCase.username, testCase.orgName, http.StatusOK)
			assert.Equal(t, testCase.expected, *DecodeJSON(t, resp, &api.OrganizationPermissions{}))
		})
	}
}
