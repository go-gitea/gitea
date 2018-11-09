// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestAPIOrg(t *testing.T) {
	prepareTestEnv(t)

	session := loginUser(t, "user1")

	token := getTokenForLoggedInUser(t, session)
	var org = api.CreateOrgOption{
		UserName:    "user1_org",
		FullName:    "User1's organization",
		Description: "This organization created by user1",
		Website:     "https://try.gitea.io",
		Location:    "Shanghai",
	}
	req := NewRequestWithJSON(t, "POST", "/api/v1/orgs?token="+token, &org)
	resp := session.MakeRequest(t, req, http.StatusCreated)

	var apiOrg api.Organization
	DecodeJSON(t, resp, &apiOrg)

	assert.Equal(t, org.UserName, apiOrg.UserName)
	assert.Equal(t, org.FullName, apiOrg.FullName)
	assert.Equal(t, org.Description, apiOrg.Description)
	assert.Equal(t, org.Website, apiOrg.Website)
	assert.Equal(t, org.Location, apiOrg.Location)

	models.AssertExistsAndLoadBean(t, &models.User{
		Name:      org.UserName,
		LowerName: strings.ToLower(org.UserName),
		FullName:  org.FullName,
	})
}
