// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIRepoBranchesPlain(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		session := loginUser(t, user1.LowerName)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		link, _ := url.Parse(fmt.Sprintf("/api/v1/repos/user3/%s/branches", repo3.Name)) // a plain repo
		link.RawQuery = url.Values{"token": {token}}.Encode()
		resp := MakeRequest(t, NewRequest(t, "GET", link.String()), http.StatusOK)
		bs, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)

		var branches []*api.Branch
		assert.NoError(t, json.Unmarshal(bs, &branches))
		assert.Len(t, branches, 2)
		sort.Slice(branches, func(i, j int) bool {
			return branches[i].Name > branches[j].Name
		})
		assert.EqualValues(t, "test_branch", branches[0].Name)
		assert.EqualValues(t, "master", branches[1].Name)

		link2, _ := url.Parse(fmt.Sprintf("/api/v1/repos/user3/%s/branches/test_branch", repo3.Name))
		link2.RawQuery = url.Values{"token": {token}}.Encode()
		resp = MakeRequest(t, NewRequest(t, "GET", link2.String()), http.StatusOK)
		bs, err = io.ReadAll(resp.Body)
		assert.NoError(t, err)
		var branch api.Branch
		assert.NoError(t, json.Unmarshal(bs, &branch))
		assert.EqualValues(t, "test_branch", branch.Name)

		req := NewRequest(t, "POST", link.String())
		req.Header.Add("Content-Type", "application/json")
		req.Body = io.NopCloser(bytes.NewBufferString(`{"new_branch_name":"test_branch2", "old_branch_name": "test_branch", "old_ref_name":"refs/heads/test_branch"}`))
		resp = MakeRequest(t, req, http.StatusCreated)
		bs, err = io.ReadAll(resp.Body)
		assert.NoError(t, err)
		var branch2 api.Branch
		assert.NoError(t, json.Unmarshal(bs, &branch2))
		assert.EqualValues(t, "test_branch2", branch2.Name)
		assert.EqualValues(t, branch.Commit.ID, branch2.Commit.ID)

		resp = MakeRequest(t, NewRequest(t, "GET", link.String()), http.StatusOK)
		bs, err = io.ReadAll(resp.Body)
		assert.NoError(t, err)

		branches = []*api.Branch{}
		assert.NoError(t, json.Unmarshal(bs, &branches))
		assert.Len(t, branches, 3)
		sort.Slice(branches, func(i, j int) bool {
			return branches[i].Name > branches[j].Name
		})
		assert.EqualValues(t, "test_branch2", branches[0].Name)
		assert.EqualValues(t, "test_branch", branches[1].Name)
		assert.EqualValues(t, "master", branches[2].Name)

		link3, _ := url.Parse(fmt.Sprintf("/api/v1/repos/user3/%s/branches/test_branch2", repo3.Name))
		MakeRequest(t, NewRequest(t, "DELETE", link3.String()), http.StatusNotFound)

		link3.RawQuery = url.Values{"token": {token}}.Encode()
		MakeRequest(t, NewRequest(t, "DELETE", link3.String()), http.StatusNoContent)
		assert.NoError(t, err)
	})
}

func TestAPIRepoBranchesMirror(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo5 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 5})
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	session := loginUser(t, user1.LowerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	link, _ := url.Parse(fmt.Sprintf("/api/v1/repos/user3/%s/branches", repo5.Name)) // a mirror repo
	link.RawQuery = url.Values{"token": {token}}.Encode()
	resp := MakeRequest(t, NewRequest(t, "GET", link.String()), http.StatusOK)
	bs, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	var branches []*api.Branch
	assert.NoError(t, json.Unmarshal(bs, &branches))
	assert.Len(t, branches, 2)
	sort.Slice(branches, func(i, j int) bool {
		return branches[i].Name > branches[j].Name
	})
	assert.EqualValues(t, "test_branch", branches[0].Name)
	assert.EqualValues(t, "master", branches[1].Name)

	link2, _ := url.Parse(fmt.Sprintf("/api/v1/repos/user3/%s/branches/test_branch", repo5.Name))
	link2.RawQuery = url.Values{"token": {token}}.Encode()
	resp = MakeRequest(t, NewRequest(t, "GET", link2.String()), http.StatusOK)
	bs, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	var branch api.Branch
	assert.NoError(t, json.Unmarshal(bs, &branch))
	assert.EqualValues(t, "test_branch", branch.Name)

	req := NewRequest(t, "POST", link.String())
	req.Header.Add("Content-Type", "application/json")
	req.Body = io.NopCloser(bytes.NewBufferString(`{"new_branch_name":"test_branch2", "old_branch_name": "test_branch", "old_ref_name":"refs/heads/test_branch"}`))
	resp = MakeRequest(t, req, http.StatusForbidden)
	bs, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.EqualValues(t, "{\"message\":\"Git Repository is a mirror.\",\"url\":\""+setting.AppURL+"api/swagger\"}\n", string(bs))

	resp = MakeRequest(t, NewRequest(t, "DELETE", link2.String()), http.StatusForbidden)
	bs, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.EqualValues(t, "{\"message\":\"Git Repository is a mirror.\",\"url\":\""+setting.AppURL+"api/swagger\"}\n", string(bs))
}
