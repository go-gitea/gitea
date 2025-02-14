// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"
	org_service "code.gitea.io/gitea/services/org"
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
	assert.True(t, exists, "Fork owner '%s' is not present in select box", forkOwnerName)
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

func TestForkListLimitedAndPrivateRepos(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	forkItemSelector := ".repo-fork-item"

	user1Sess := loginUser(t, "user1")
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user1"})

	// fork to a limited org
	limitedOrg := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 22})
	assert.EqualValues(t, structs.VisibleTypeLimited, limitedOrg.Visibility)
	ownerTeam1, err := org_model.OrgFromUser(limitedOrg).GetOwnerTeam(db.DefaultContext)
	assert.NoError(t, err)
	assert.NoError(t, org_service.AddTeamMember(db.DefaultContext, ownerTeam1, user1))
	testRepoFork(t, user1Sess, "user2", "repo1", limitedOrg.Name, "repo1", "")

	// fork to a private org
	user4Sess := loginUser(t, "user4")
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user4"})
	privateOrg := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 23})
	assert.EqualValues(t, structs.VisibleTypePrivate, privateOrg.Visibility)
	ownerTeam2, err := org_model.OrgFromUser(privateOrg).GetOwnerTeam(db.DefaultContext)
	assert.NoError(t, err)
	assert.NoError(t, org_service.AddTeamMember(db.DefaultContext, ownerTeam2, user4))
	testRepoFork(t, user4Sess, "user2", "repo1", privateOrg.Name, "repo1", "")

	t.Run("Anonymous", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", "/user2/repo1/forks")
		resp := MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.EqualValues(t, 0, htmlDoc.Find(forkItemSelector).Length())
	})

	t.Run("Logged in", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2/repo1/forks")
		resp := user1Sess.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		// since user1 is an admin, he can get both of the forked repositories
		assert.EqualValues(t, 2, htmlDoc.Find(forkItemSelector).Length())

		assert.NoError(t, org_service.AddTeamMember(db.DefaultContext, ownerTeam2, user1))
		resp = user1Sess.MakeRequest(t, req, http.StatusOK)
		htmlDoc = NewHTMLParser(t, resp.Body)
		assert.EqualValues(t, 2, htmlDoc.Find(forkItemSelector).Length())
	})
}
