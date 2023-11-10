// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

type apiUserOrgPermTestCase struct {
	LoginUser                       string
	User                            string
	Organization                    string
	ExpectedOrganizationPermissions api.OrganizationPermissions
}

func TestTokenNeeded(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/api/v1/users/user1/orgs/org6/permissions")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func sampleTest(t *testing.T, auoptc apiUserOrgPermTestCase) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, auoptc.LoginUser)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadUser)

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/orgs/%s/permissions?token=%s", auoptc.User, auoptc.Organization, token))
	resp := MakeRequest(t, req, http.StatusOK)

	var apiOP api.OrganizationPermissions
	DecodeJSON(t, resp, &apiOP)
	assert.Equal(t, auoptc.ExpectedOrganizationPermissions.IsOwner, apiOP.IsOwner)
	assert.Equal(t, auoptc.ExpectedOrganizationPermissions.IsAdmin, apiOP.IsAdmin)
	assert.Equal(t, auoptc.ExpectedOrganizationPermissions.CanWrite, apiOP.CanWrite)
	assert.Equal(t, auoptc.ExpectedOrganizationPermissions.CanRead, apiOP.CanRead)
	assert.Equal(t, auoptc.ExpectedOrganizationPermissions.CanCreateRepository, apiOP.CanCreateRepository)
}

func TestWithOwnerUser(t *testing.T) {
	sampleTest(t, apiUserOrgPermTestCase{
		LoginUser:    "user2",
		User:         "user2",
		Organization: "org3",
		ExpectedOrganizationPermissions: api.OrganizationPermissions{
			IsOwner:             true,
			IsAdmin:             true,
			CanWrite:            true,
			CanRead:             true,
			CanCreateRepository: true,
		},
	})
}

func TestCanWriteUser(t *testing.T) {
	sampleTest(t, apiUserOrgPermTestCase{
		LoginUser:    "user4",
		User:         "user4",
		Organization: "org3",
		ExpectedOrganizationPermissions: api.OrganizationPermissions{
			IsOwner:             false,
			IsAdmin:             false,
			CanWrite:            true,
			CanRead:             true,
			CanCreateRepository: false,
		},
	})
}

func TestAdminUser(t *testing.T) {
	sampleTest(t, apiUserOrgPermTestCase{
		LoginUser:    "user1",
		User:         "user28",
		Organization: "org3",
		ExpectedOrganizationPermissions: api.OrganizationPermissions{
			IsOwner:             false,
			IsAdmin:             true,
			CanWrite:            true,
			CanRead:             true,
			CanCreateRepository: true,
		},
	})
}

func TestAdminCanNotCreateRepo(t *testing.T) {
	sampleTest(t, apiUserOrgPermTestCase{
		LoginUser:    "user1",
		User:         "user28",
		Organization: "org6",
		ExpectedOrganizationPermissions: api.OrganizationPermissions{
			IsOwner:             false,
			IsAdmin:             true,
			CanWrite:            true,
			CanRead:             true,
			CanCreateRepository: false,
		},
	})
}

func TestCanReadUser(t *testing.T) {
	sampleTest(t, apiUserOrgPermTestCase{
		LoginUser:    "user1",
		User:         "user24",
		Organization: "org25",
		ExpectedOrganizationPermissions: api.OrganizationPermissions{
			IsOwner:             false,
			IsAdmin:             false,
			CanWrite:            false,
			CanRead:             true,
			CanCreateRepository: false,
		},
	})
}

func TestUnknowUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadOrganization)

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/unknow/orgs/org25/permissions?token=%s", token))
	resp := MakeRequest(t, req, http.StatusNotFound)

	var apiError api.APIError
	DecodeJSON(t, resp, &apiError)
	assert.Equal(t, "user redirect does not exist [name: unknow]", apiError.Message)
}

func TestUnknowOrganization(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadOrganization)

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/user1/orgs/unknow/permissions?token=%s", token))
	resp := MakeRequest(t, req, http.StatusNotFound)
	var apiError api.APIError
	DecodeJSON(t, resp, &apiError)
	assert.Equal(t, "GetUserByName", apiError.Message)
}
