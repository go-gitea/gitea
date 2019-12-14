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
