// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func testAPIGetBranch(t *testing.T, branchName string, exists bool) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/branches/%s?token=%s", branchName, token)
	resp := session.MakeRequest(t, req, NoExpectedStatus)
	if !exists {
		assert.EqualValues(t, http.StatusNotFound, resp.Code)
		return
	}
	assert.EqualValues(t, http.StatusOK, resp.Code)
	var branch api.Branch
	DecodeJSON(t, resp, &branch)
	assert.EqualValues(t, branchName, branch.Name)
}

func testAPIGetBranchProtection(t *testing.T, branchName string, expectedHTTPStatus int) {
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/branch_protections/%s?token=%s", branchName, token)
	resp := session.MakeRequest(t, req, expectedHTTPStatus)

	if resp.Code == 200 {
		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.EqualValues(t, branchName, branchProtection.BranchName)
	}
}

func testAPICreateBranchProtection(t *testing.T, branchName string, expectedHTTPStatus int) {
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/branch_protections?token="+token, &api.BranchProtection{
		BranchName: branchName,
	})
	resp := session.MakeRequest(t, req, expectedHTTPStatus)

	if resp.Code == 201 {
		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.EqualValues(t, branchName, branchProtection.BranchName)
	}
}

func testAPIEditBranchProtection(t *testing.T, branchName string, body *api.BranchProtection, expectedHTTPStatus int) {
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestWithJSON(t, "PATCH", "/api/v1/repos/user2/repo1/branch_protections/"+branchName+"?token="+token, body)
	resp := session.MakeRequest(t, req, expectedHTTPStatus)

	if resp.Code == 200 {
		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.EqualValues(t, branchName, branchProtection.BranchName)
	}
}

func testAPIDeleteBranchProtection(t *testing.T, branchName string, expectedHTTPStatus int) {
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "DELETE", "/api/v1/repos/user2/repo1/branch_protections/%s?token=%s", branchName, token)
	session.MakeRequest(t, req, expectedHTTPStatus)
}

func TestAPIGetBranch(t *testing.T) {
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

func TestAPIBranchProtection(t *testing.T) {
	defer prepareTestEnv(t)()

	// Branch protection only on branch that exist
	testAPICreateBranchProtection(t, "master/doesnotexist", http.StatusNotFound)
	// Get branch protection on branch that exist but not branch protection
	testAPIGetBranchProtection(t, "master", http.StatusNotFound)

	testAPICreateBranchProtection(t, "master", http.StatusCreated)
	// Can only create once
	testAPICreateBranchProtection(t, "master", http.StatusForbidden)

	testAPIGetBranchProtection(t, "master", http.StatusOK)
	testAPIEditBranchProtection(t, "master", &api.BranchProtection{
		EnablePush: true,
	}, http.StatusOK)

	testAPIDeleteBranchProtection(t, "master", http.StatusNoContent)
}
