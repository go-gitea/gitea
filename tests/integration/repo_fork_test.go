// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func testRepoFork(t *testing.T, session *TestSession, ownerName, repoName, forkOwnerName, forkRepoName, forkBranch string) *httptest.ResponseRecorder {
	forkOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: forkOwnerName})

	// Step0: check the existence of the to-fork repo
	req := NewRequestf(t, "GET", "/%s/%s", forkOwnerName, forkRepoName)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Step1: go to the main page of repo
	req = NewRequestf(t, "GET", "/%s/%s", ownerName, repoName)
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Step2: click the fork button
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find(`a.ui.button[href*="/fork"]`).Attr("href")
	assert.True(t, exists, "The template has changed")
	req = NewRequest(t, "GET", link)
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Step3: fill the form of the forking
	htmlDoc = NewHTMLParser(t, resp.Body)
	link, exists = htmlDoc.doc.Find(`form.ui.form[action*="/fork"]`).Attr("action")
	assert.True(t, exists, "The template has changed")
	_, exists = htmlDoc.doc.Find(fmt.Sprintf(".owner.dropdown .item[data-value=\"%d\"]", forkOwner.ID)).Attr("data-value")
	assert.True(t, exists, fmt.Sprintf("Fork owner '%s' is not present in select box", forkOwnerName))
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf":              htmlDoc.GetCSRF(),
		"uid":                fmt.Sprintf("%d", forkOwner.ID),
		"repo_name":          forkRepoName,
		"fork_single_branch": forkBranch,
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	// Step4: check the existence of the forked repo
	req = NewRequestf(t, "GET", "/%s/%s", forkOwnerName, forkRepoName)
	resp = session.MakeRequest(t, req, http.StatusOK)

	return resp
}

func TestRepoFork(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")
	testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
}

func TestRepoForkToOrg(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	testRepoFork(t, session, "user2", "repo1", "org3", "repo1", "")

	// Check that no more forking is allowed as user2 owns repository
	//  and org3 organization that owner user2 is also now has forked this repository
	req := NewRequest(t, "GET", "/user2/repo1")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	_, exists := htmlDoc.doc.Find(`a.ui.button[href*="/fork"]`).Attr("href")
	assert.False(t, exists, "Forking should not be allowed anymore")
}
