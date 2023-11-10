// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"
	"github.com/stretchr/testify/assert"
)

func TestAPIListUserProjects(t *testing.T) {
}

func TestAPIListOrgProjects(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 17})

	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadIssue)
	link, _ := url.Parse(fmt.Sprintf("/api/v1/orgs/%s/projects", org.Name))

	link.RawQuery = url.Values{"token": { token}}.Encode()

	req := NewRequest(t, "GET", link.String())
	var apiProjects []*api.Project

	resp := session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiProjects)
	assert.Len(t, apiProjects, 1)

}

func TestAPIListRepoProjects(t *testing.T) {
}

func TestAPICreateUserProject(t *testing.T) {
}

func TestAPICreateOrgProject(t *testing.T) {
}

func TestAPICreateRepoProject(t *testing.T) {
}

func TestAPIGetProject(t *testing.T) {
}

func TestAPIUpdateProject(t *testing.T) {
}

func TestAPIDeleteProject(t *testing.T) {
}
