// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
)

func TestAPIReposGitTrees(t *testing.T) {
	defer prepareTestEnv(t)()
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)               // owner of the repo1 & repo16
	user3 := models.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User)               // owner of the repo3
	user4 := models.AssertExistsAndLoadBean(t, &models.User{ID: 4}).(*models.User)               // owner of neither repos
	repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)   // public repo
	repo3 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)   // public repo
	repo16 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 16}).(*models.Repository) // private repo
	repo1TreeSHA := "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	repo3TreeSHA := "2a47ca4b614a9f5a43abbd5ad851a54a616ffee6"
	repo16TreeSHA := "69554a64c1e6030f051e5c3f94bfbd773cd6a324"
	badSHA := "0000000000000000000000000000000000000000"

	// Login as User2.
	session := loginUser(t, user2.Name)
	token := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t) // don't want anyone logged in for this

	// Test a public repo that anyone can GET the tree of
	for _, ref := range [...]string{
		"master",     // Branch
		repo1TreeSHA, // Tree SHA
	} {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/trees/%s", user2.Name, repo1.Name, ref)
		session.MakeRequest(t, req, http.StatusOK)
	}

	// Tests a private repo with no token so will fail
	for _, ref := range [...]string{
		"master",     // Branch
		repo1TreeSHA, // Tag
	} {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/trees/%s", user2.Name, repo16.Name, ref)
		session.MakeRequest(t, req, http.StatusNotFound)
	}

	// Test using access token for a private repo that the user of the token owns
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/trees/%s?token=%s", user2.Name, repo16.Name, repo16TreeSHA, token)
	session.MakeRequest(t, req, http.StatusOK)

	// Test using bad sha
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/trees/%s", user2.Name, repo1.Name, badSHA)
	session.MakeRequest(t, req, http.StatusBadRequest)

	// Test using org repo "user3/repo3" where user2 is a collaborator
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/trees/%s?token=%s", user3.Name, repo3.Name, repo3TreeSHA, token)
	session.MakeRequest(t, req, http.StatusOK)

	// Test using org repo "user3/repo3" with no user token
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/trees/%s", user3.Name, repo3TreeSHA, repo3.Name)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Login as User4.
	session = loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t) // don't want anyone logged in for this

	// Test using org repo "user3/repo3" where user4 is a NOT collaborator
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/trees/d56a3073c1dbb7b15963110a049d50cdb5db99fc?access=%s", user3.Name, repo3.Name, token4)
	session.MakeRequest(t, req, http.StatusNotFound)
}
