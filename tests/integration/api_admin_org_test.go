// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIAdminOrgCreate(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin)

	for _, testCase := range []struct {
		name       string
		visibility api.VisibilityString
		status     int
	}{
		{"CreateOrg", "private", http.StatusCreated},
		{"CreateBadVisibility", "notvalid", http.StatusUnprocessableEntity},
		{"CreateNotAdmin", "public", http.StatusForbidden},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			testToken := token
			if testCase.status == http.StatusForbidden {
				session := loginUser(t, "user2")
				testToken = getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeAll)
			}
			org := api.CreateOrgOption{
				UserName:    "user2_org",
				FullName:    "User2's organization",
				Description: "This organization created by admin for user2",
				Website:     "https://try.gitea.io",
				Location:    "Shanghai",
				Visibility:  testCase.visibility,
			}
			req := NewRequestWithJSON(t, "POST", "/api/v1/admin/users/user2/orgs", &org).
				AddTokenAuth(testToken)
			resp := MakeRequest(t, req, testCase.status)
			if testCase.status == http.StatusCreated {
				apiOrg := DecodeJSON(t, resp, &api.Organization{})
				assert.Equal(t, org.UserName, apiOrg.Name)
				assert.Equal(t, org.FullName, apiOrg.FullName)
				assert.Equal(t, org.Description, apiOrg.Description)
				assert.Equal(t, org.Website, apiOrg.Website)
				assert.Equal(t, org.Location, apiOrg.Location)
				assert.Equal(t, org.Visibility, apiOrg.Visibility)

				unittest.AssertExistsAndLoadBean(t, &user_model.User{
					Name:      org.UserName,
					LowerName: strings.ToLower(org.UserName),
					FullName:  org.FullName,
				})
			}
		})
	}
}
