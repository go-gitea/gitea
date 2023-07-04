// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestRenameBranch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: 1, Name: "master"})

	// get branch setting page
	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/settings/branches")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	postData := map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"from":  "master",
		"to":    "main",
	}
	req = NewRequestWithValues(t, "POST", "/user2/repo1/settings/rename_branch", postData)
	session.MakeRequest(t, req, http.StatusSeeOther)

	// check new branch link
	req = NewRequestWithValues(t, "GET", "/user2/repo1/src/branch/main/README.md", postData)
	session.MakeRequest(t, req, http.StatusOK)

	// check old branch link
	req = NewRequestWithValues(t, "GET", "/user2/repo1/src/branch/master/README.md", postData)
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	location := resp.Header().Get("Location")
	assert.Equal(t, "/user2/repo1/src/branch/main/README.md", location)

	// check db
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Equal(t, "main", repo1.DefaultBranch)
}
