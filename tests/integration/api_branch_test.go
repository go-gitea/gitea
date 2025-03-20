// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func testAPIGetBranch(t *testing.T, branchName string, exists bool) {
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)
	req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/branches/%s", branchName).
		AddTokenAuth(token)
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

func testAPIGetBranchProtection(t *testing.T, branchName string, expectedHTTPStatus int) *api.BranchProtection {
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)
	req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/branch_protections/%s", branchName).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, expectedHTTPStatus)

	if resp.Code == http.StatusOK {
		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.EqualValues(t, branchName, branchProtection.RuleName)
		return &branchProtection
	}
	return nil
}

func testAPICreateBranchProtection(t *testing.T, branchName string, expectedPriority, expectedHTTPStatus int) {
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/branch_protections", &api.BranchProtection{
		RuleName: branchName,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, expectedHTTPStatus)

	if resp.Code == http.StatusCreated {
		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.EqualValues(t, branchName, branchProtection.RuleName)
		assert.EqualValues(t, expectedPriority, branchProtection.Priority)
	}
}

func testAPIEditBranchProtection(t *testing.T, branchName string, body *api.BranchProtection, expectedHTTPStatus int) {
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, "PATCH", "/api/v1/repos/user2/repo1/branch_protections/"+branchName, body).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, expectedHTTPStatus)

	if resp.Code == http.StatusOK {
		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.EqualValues(t, branchName, branchProtection.RuleName)
	}
}

func testAPIDeleteBranchProtection(t *testing.T, branchName string, expectedHTTPStatus int) {
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestf(t, "DELETE", "/api/v1/repos/user2/repo1/branch_protections/%s", branchName).
		AddTokenAuth(token)
	MakeRequest(t, req, expectedHTTPStatus)
}

func testAPIDeleteBranch(t *testing.T, branchName string, expectedHTTPStatus int) {
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestf(t, "DELETE", "/api/v1/repos/user2/repo1/branches/%s", branchName).
		AddTokenAuth(token)
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
		// ps: it can't test the case-sensitive behavior here: the "BRANCH_2" can't be created by git on a case-insensitive filesystem, it makes the test fail quickly before the database code.
		// Suppose some users are running Gitea on a case-insensitive filesystem, it seems that it's unable to support case-sensitive branch names.
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
		// Trying to create a branch with UTF8
		{
			OldBranch:          "master",
			NewBranch:          "test-👀",
			ExpectedHTTPStatus: http.StatusCreated,
		},
	}
	for _, test := range testCases {
		session := ctx.Session
		t.Run(test.NewBranch, func(t *testing.T) {
			testAPICreateBranch(t, session, "user2", "my-noo-repo", test.OldBranch, test.NewBranch, test.ExpectedHTTPStatus)
		})
	}
}

func testAPICreateBranch(t testing.TB, session *TestSession, user, repo, oldBranch, newBranch string, status int) bool {
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/"+user+"/"+repo+"/branches", &api.CreateBranchRepoOption{
		BranchName:    newBranch,
		OldBranchName: oldBranch,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, status)

	var branch api.Branch
	DecodeJSON(t, resp, &branch)

	if resp.Result().StatusCode == http.StatusCreated {
		assert.EqualValues(t, newBranch, branch.Name)
	}

	return resp.Result().StatusCode == status
}

func TestAPIUpdateBranch(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		t.Run("UpdateBranchWithEmptyRepo", func(t *testing.T) {
			testAPIUpdateBranch(t, "user10", "user10", "repo6", "master", "test", http.StatusNotFound)
		})
		t.Run("UpdateBranchWithSameBranchNames", func(t *testing.T) {
			resp := testAPIUpdateBranch(t, "user2", "user2", "repo1", "master", "master", http.StatusUnprocessableEntity)
			assert.Contains(t, resp.Body.String(), "Cannot rename a branch using the same name or rename to a branch that already exists.")
		})
		t.Run("UpdateBranchThatAlreadyExists", func(t *testing.T) {
			resp := testAPIUpdateBranch(t, "user2", "user2", "repo1", "master", "branch2", http.StatusUnprocessableEntity)
			assert.Contains(t, resp.Body.String(), "Cannot rename a branch using the same name or rename to a branch that already exists.")
		})
		t.Run("UpdateBranchWithNonExistentBranch", func(t *testing.T) {
			resp := testAPIUpdateBranch(t, "user2", "user2", "repo1", "i-dont-exist", "new-branch-name", http.StatusNotFound)
			assert.Contains(t, resp.Body.String(), "Branch doesn't exist.")
		})
		t.Run("UpdateBranchWithNonAdminDoer", func(t *testing.T) {
			// don't allow default branch renaming
			resp := testAPIUpdateBranch(t, "user40", "user2", "repo1", "master", "new-branch-name", http.StatusForbidden)
			assert.Contains(t, resp.Body.String(), "User must be a repo or site admin to rename default or protected branches.")

			// don't allow protected branch renaming
			token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
			req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/branches", &api.CreateBranchRepoOption{
				BranchName: "protected-branch",
			}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
			testAPICreateBranchProtection(t, "protected-branch", 1, http.StatusCreated)
			resp = testAPIUpdateBranch(t, "user40", "user2", "repo1", "protected-branch", "new-branch-name", http.StatusForbidden)
			assert.Contains(t, resp.Body.String(), "User must be a repo or site admin to rename default or protected branches.")
		})
		t.Run("UpdateBranchWithGlobedBasedProtectionRulesAndAdminAccess", func(t *testing.T) {
			// don't allow branch that falls under glob-based protection rules to be renamed
			token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
			req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/branch_protections", &api.BranchProtection{
				RuleName:   "protected/**",
				EnablePush: true,
			}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)

			from := "protected/1"
			req = NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/branches", &api.CreateBranchRepoOption{
				BranchName: from,
			}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)

			resp := testAPIUpdateBranch(t, "user2", "user2", "repo1", from, "new-branch-name", http.StatusForbidden)
			assert.Contains(t, resp.Body.String(), "Branch is protected by glob-based protection rules.")
		})
		t.Run("UpdateBranchNormalScenario", func(t *testing.T) {
			testAPIUpdateBranch(t, "user2", "user2", "repo1", "branch2", "new-branch-name", http.StatusNoContent)
		})
	})
}

func testAPIUpdateBranch(t *testing.T, doerName, ownerName, repoName, from, to string, expectedHTTPStatus int) *httptest.ResponseRecorder {
	token := getUserToken(t, doerName, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, "PATCH", "api/v1/repos/"+ownerName+"/"+repoName+"/branches/"+from, &api.UpdateBranchRepoOption{
		Name: to,
	}).AddTokenAuth(token)
	return MakeRequest(t, req, expectedHTTPStatus)
}

func TestAPIBranchProtection(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Branch protection  on branch that not exist
	testAPICreateBranchProtection(t, "master/doesnotexist", 1, http.StatusCreated)
	// Get branch protection on branch that exist but not branch protection
	testAPIGetBranchProtection(t, "master", http.StatusNotFound)

	testAPICreateBranchProtection(t, "master", 2, http.StatusCreated)
	// Can only create once
	testAPICreateBranchProtection(t, "master", 0, http.StatusForbidden)

	// Can't delete a protected branch
	testAPIDeleteBranch(t, "master", http.StatusForbidden)

	testAPIGetBranchProtection(t, "master", http.StatusOK)
	testAPIEditBranchProtection(t, "master", &api.BranchProtection{
		EnablePush: true,
	}, http.StatusOK)

	// enable status checks, require the "test1" check to pass
	testAPIEditBranchProtection(t, "master", &api.BranchProtection{
		EnableStatusCheck:   true,
		StatusCheckContexts: []string{"test1"},
	}, http.StatusOK)
	bp := testAPIGetBranchProtection(t, "master", http.StatusOK)
	assert.True(t, bp.EnableStatusCheck)
	assert.Equal(t, []string{"test1"}, bp.StatusCheckContexts)

	// disable status checks, clear the list of required checks
	testAPIEditBranchProtection(t, "master", &api.BranchProtection{
		EnableStatusCheck:   false,
		StatusCheckContexts: []string{},
	}, http.StatusOK)
	bp = testAPIGetBranchProtection(t, "master", http.StatusOK)
	assert.False(t, bp.EnableStatusCheck)
	assert.Equal(t, []string{}, bp.StatusCheckContexts)

	testAPIDeleteBranchProtection(t, "master", http.StatusNoContent)

	// Test branch deletion
	testAPIDeleteBranch(t, "master", http.StatusForbidden)
	testAPIDeleteBranch(t, "branch2", http.StatusNoContent)
}

func TestAPICreateBranchWithSyncBranches(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	branches, err := db.Find[git_model.Branch](db.DefaultContext, git_model.FindBranchOptions{
		RepoID: 1,
	})
	assert.NoError(t, err)
	assert.Len(t, branches, 4)

	// make a broke repository with no branch on database
	_, err = db.DeleteByBean(db.DefaultContext, git_model.Branch{RepoID: 1})
	assert.NoError(t, err)

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		ctx := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		giteaURL.Path = ctx.GitPath()

		testAPICreateBranch(t, ctx.Session, "user2", "repo1", "", "new_branch", http.StatusCreated)
	})

	branches, err = db.Find[git_model.Branch](db.DefaultContext, git_model.FindBranchOptions{
		RepoID: 1,
	})
	assert.NoError(t, err)
	assert.Len(t, branches, 5)

	branches, err = db.Find[git_model.Branch](db.DefaultContext, git_model.FindBranchOptions{
		RepoID:  1,
		Keyword: "new_branch",
	})
	assert.NoError(t, err)
	assert.Len(t, branches, 1)
}
