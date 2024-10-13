// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	gitea_context "code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestRenameBranch(t *testing.T) {
	onGiteaRun(t, testRenameBranch)
}

func testRenameBranch(t *testing.T, u *url.URL) {
	defer tests.PrepareTestEnv(t)()

	unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: 1, Name: "master"})

	// get branch setting page
	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/branches")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	req = NewRequestWithValues(t, "POST", "/user2/repo1/branches/rename", map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"from":  "master",
		"to":    "main",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	// check new branch link
	req = NewRequestWithValues(t, "GET", "/user2/repo1/src/branch/main/README.md", nil)
	session.MakeRequest(t, req, http.StatusOK)

	// check old branch link
	req = NewRequestWithValues(t, "GET", "/user2/repo1/src/branch/master/README.md", nil)
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	location := resp.Header().Get("Location")
	assert.Equal(t, "/user2/repo1/src/branch/main/README.md", location)

	// check db
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Equal(t, "main", repo1.DefaultBranch)

	// create branch1
	csrf := GetUserCSRFToken(t, session)

	req = NewRequestWithValues(t, "POST", "/user2/repo1/branches/_new/branch/main", map[string]string{
		"_csrf":           csrf,
		"new_branch_name": "branch1",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	branch1 := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo1.ID, Name: "branch1"})
	assert.Equal(t, "branch1", branch1.Name)

	// create branch2
	req = NewRequestWithValues(t, "POST", "/user2/repo1/branches/_new/branch/main", map[string]string{
		"_csrf":           csrf,
		"new_branch_name": "branch2",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	branch2 := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo1.ID, Name: "branch2"})
	assert.Equal(t, "branch2", branch2.Name)

	// rename branch2 to branch1
	req = NewRequestWithValues(t, "POST", "/user2/repo1/branches/rename", map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"from":  "branch2",
		"to":    "branch1",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
	flashCookie := session.GetCookie(gitea_context.CookieNameFlash)
	assert.NotNil(t, flashCookie)
	assert.Contains(t, flashCookie.Value, "error")

	branch2 = unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo1.ID, Name: "branch2"})
	assert.Equal(t, "branch2", branch2.Name)
	branch1 = unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo1.ID, Name: "branch1"})
	assert.Equal(t, "branch1", branch1.Name)

	// delete branch1
	req = NewRequestWithValues(t, "POST", "/user2/repo1/branches/delete", map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"name":  "branch1",
	})
	session.MakeRequest(t, req, http.StatusOK)
	branch2 = unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo1.ID, Name: "branch2"})
	assert.Equal(t, "branch2", branch2.Name)
	branch1 = unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo1.ID, Name: "branch1"})
	assert.True(t, branch1.IsDeleted) // virtual deletion

	// rename branch2 to branch1 again
	req = NewRequestWithValues(t, "POST", "/user2/repo1/branches/rename", map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"from":  "branch2",
		"to":    "branch1",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	flashCookie = session.GetCookie(gitea_context.CookieNameFlash)
	assert.NotNil(t, flashCookie)
	assert.Contains(t, flashCookie.Value, "success")

	unittest.AssertNotExistsBean(t, &git_model.Branch{RepoID: repo1.ID, Name: "branch2"})
	branch1 = unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo1.ID, Name: "branch1"})
	assert.Equal(t, "branch1", branch1.Name)
}
