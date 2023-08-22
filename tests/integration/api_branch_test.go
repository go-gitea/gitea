// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func testAPIGetBranch(t *testing.T, branchName string, exists bool) {
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)
	req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/branches/%s?token=%s", branchName, token)
	resp := MakeRequest(t, req, NoExpectedStatus)
	if !exists {
		assert.EqualValues(t, http.StatusNotFound, resp.Code)
		return
	}
	assert.EqualValues(t, http.StatusOK, resp.Code)
	var branch api.Branch
	DecodeJSON(t, resp, &branch)
	assert.EqualValues(t, branchName, branch.Name)
	assert.True(t, branch.UserCanPush)
	assert.True(t, branch.UserCanMerge)
}

func testAPIGetBranchProtection(t *testing.T, branchName string, expectedHTTPStatus int) {
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)
	req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/branch_protections/%s?token=%s", branchName, token)
	resp := MakeRequest(t, req, expectedHTTPStatus)

	if resp.Code == http.StatusOK {
		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.EqualValues(t, branchName, branchProtection.RuleName)
	}
}

func testAPICreateBranchProtection(t *testing.T, branchName string, expectedHTTPStatus int) {
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/branch_protections?token="+token, &api.BranchProtection{
		RuleName: branchName,
	})
	resp := MakeRequest(t, req, expectedHTTPStatus)

	if resp.Code == http.StatusCreated {
		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.EqualValues(t, branchName, branchProtection.RuleName)
	}
}

func testAPIEditBranchProtection(t *testing.T, branchName string, body *api.BranchProtection, expectedHTTPStatus int) {
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, "PATCH", "/api/v1/repos/user2/repo1/branch_protections/"+branchName+"?token="+token, body)
	resp := MakeRequest(t, req, expectedHTTPStatus)

	if resp.Code == http.StatusOK {
		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.EqualValues(t, branchName, branchProtection.RuleName)
	}
}

func testAPIDeleteBranchProtection(t *testing.T, branchName string, expectedHTTPStatus int) {
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestf(t, "DELETE", "/api/v1/repos/user2/repo1/branch_protections/%s?token=%s", branchName, token)
	MakeRequest(t, req, expectedHTTPStatus)
}

func testAPIDeleteBranch(t *testing.T, branchName string, expectedHTTPStatus int) {
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestf(t, "DELETE", "/api/v1/repos/user2/repo1/branches/%s?token=%s", branchName, token)
	MakeRequest(t, req, expectedHTTPStatus)
}

func TestAPIGetBranch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	for _, test := range []struct {
		BranchName string
		Exists     bool
	}{
		{"master", true},
		{"master/doesnotexist", false},
		{"feature/1", true},
		{"feature/1/doesnotexist", false},
	} {
		testAPIGetBranch(t, test.BranchName, test.Exists)
	}
}

func TestAPICreateBranch(t *testing.T) {
	onGiteaRun(t, testAPICreateBranches)
}

func testAPICreateBranches(t *testing.T, giteaURL *url.URL) {
	username := "user2"
	ctx := NewAPITestContext(t, username, "my-noo-repo", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
	giteaURL.Path = ctx.GitPath()

	t.Run("CreateRepo", doAPICreateRepository(ctx, false))
	testCases := []struct {
		OldBranch          string
		NewBranch          string
		ExpectedHTTPStatus int
	}{
		// Creating branch from default branch
		{
			OldBranch:          "",
			NewBranch:          "new_branch_from_default_branch",
			ExpectedHTTPStatus: http.StatusCreated,
		},
		// Creating branch from master
		{
			OldBranch:          "master",
			NewBranch:          "new_branch_from_master_1",
			ExpectedHTTPStatus: http.StatusCreated,
		},
		// Trying to create from master but already exists
		{
			OldBranch:          "master",
			NewBranch:          "new_branch_from_master_1",
			ExpectedHTTPStatus: http.StatusConflict,
		},
		// Trying to create from other branch (not default branch)
		{
			OldBranch:          "new_branch_from_master_1",
			NewBranch:          "branch_2",
			ExpectedHTTPStatus: http.StatusCreated,
		},
		// Trying to create from a branch which does not exist
		{
			OldBranch:          "does_not_exist",
			NewBranch:          "new_branch_from_non_existent",
			ExpectedHTTPStatus: http.StatusNotFound,
		},
	}
	for _, test := range testCases {
		session := ctx.Session
		testAPICreateBranch(t, session, "user2", "my-noo-repo", test.OldBranch, test.NewBranch, test.ExpectedHTTPStatus)
	}
}

func testAPICreateBranch(t testing.TB, session *TestSession, user, repo, oldBranch, newBranch string, status int) bool {
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/"+user+"/"+repo+"/branches?token="+token, &api.CreateBranchRepoOption{
		BranchName:    newBranch,
		OldBranchName: oldBranch,
	})
	resp := MakeRequest(t, req, status)

	var branch api.Branch
	DecodeJSON(t, resp, &branch)

	if status == http.StatusCreated {
		assert.EqualValues(t, newBranch, branch.Name)
	}

	return resp.Result().StatusCode == status
}

func TestAPIBranchProtection(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Branch protection  on branch that not exist
	testAPICreateBranchProtection(t, "master/doesnotexist", http.StatusCreated)
	// Get branch protection on branch that exist but not branch protection
	testAPIGetBranchProtection(t, "master", http.StatusNotFound)

	testAPICreateBranchProtection(t, "master", http.StatusCreated)
	// Can only create once
	testAPICreateBranchProtection(t, "master", http.StatusForbidden)

	// Can't delete a protected branch
	testAPIDeleteBranch(t, "master", http.StatusForbidden)

	testAPIGetBranchProtection(t, "master", http.StatusOK)
	testAPIEditBranchProtection(t, "master", &api.BranchProtection{
		EnablePush: true,
	}, http.StatusOK)

	testAPIDeleteBranchProtection(t, "master", http.StatusNoContent)

	// Test branch deletion
	testAPIDeleteBranch(t, "master", http.StatusForbidden)
	testAPIDeleteBranch(t, "branch2", http.StatusNoContent)
}
