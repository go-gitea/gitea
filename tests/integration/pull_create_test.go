// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func testPullCreate(t *testing.T, session *TestSession, user, repo string, toSelf bool, targetBranch, sourceBranch, title string) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", path.Join(user, repo))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Click the PR button to create a pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("#new-pull-request").Attr("href")
	assert.True(t, exists, "The template has changed")

	targetUser := strings.Split(link, "/")[1]
	if toSelf && targetUser != user {
		link = strings.Replace(link, targetUser, user, 1)
	}

	if targetBranch != "master" {
		link = strings.Replace(link, "master...", targetBranch+"...", 1)
	}
	if sourceBranch != "master" {
		if targetUser == user {
			link = strings.Replace(link, "...master", "..."+sourceBranch, 1)
		} else {
			link = strings.Replace(link, ":master", ":"+sourceBranch, 1)
		}
	}

	req = NewRequest(t, "GET", link)
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Submit the form for creating the pull
	htmlDoc = NewHTMLParser(t, resp.Body)
	link, exists = htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"title": title,
	})
	resp = session.MakeRequest(t, req, http.StatusOK)
	return resp
}

func testPullCreateDirectly(t *testing.T, session *TestSession, baseRepoOwner, baseRepoName, baseBranch, headRepoOwner, headRepoName, headBranch, title string) *httptest.ResponseRecorder {
	headCompare := headBranch
	if headRepoOwner != "" {
		if headRepoName != "" {
			headCompare = fmt.Sprintf("%s/%s:%s", headRepoOwner, headRepoName, headBranch)
		} else {
			headCompare = fmt.Sprintf("%s:%s", headRepoOwner, headBranch)
		}
	}
	req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/compare/%s...%s", baseRepoOwner, baseRepoName, baseBranch, headCompare))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Submit the form for creating the pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"title": title,
	})
	resp = session.MakeRequest(t, req, http.StatusOK)
	return resp
}

func testPullCreateFailure(t *testing.T, session *TestSession, baseRepoOwner, baseRepoName, baseBranch, headRepoOwner, headRepoName, headBranch, title string) *httptest.ResponseRecorder {
	headCompare := headBranch
	if headRepoOwner != "" {
		if headRepoName != "" {
			headCompare = fmt.Sprintf("%s/%s:%s", headRepoOwner, headRepoName, headBranch)
		} else {
			headCompare = fmt.Sprintf("%s:%s", headRepoOwner, headBranch)
		}
	}
	req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/compare/%s...%s", baseRepoOwner, baseRepoName, baseBranch, headCompare))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Submit the form for creating the pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"title": title,
	})
	resp = session.MakeRequest(t, req, http.StatusBadRequest)
	return resp
}

func TestPullCreate(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "This is a pull title")

		// check the redirected URL
		url := test.RedirectURL(resp)
		assert.Regexp(t, "^/user2/repo1/pulls/[0-9]*$", url)

		// check .diff can be accessed and matches performed change
		req := NewRequest(t, "GET", url+".diff")
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Regexp(t, `\+Hello, World \(Edited\)`, resp.Body)
		assert.Regexp(t, "^diff", resp.Body)
		assert.NotRegexp(t, "diff.*diff", resp.Body) // not two diffs, just one

		// check .patch can be accessed and matches performed change
		req = NewRequest(t, "GET", url+".patch")
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Regexp(t, `\+Hello, World \(Edited\)`, resp.Body)
		assert.Regexp(t, "diff", resp.Body)
		assert.Regexp(t, `Subject: \[PATCH\] Update README.md`, resp.Body)
		assert.NotRegexp(t, "diff.*diff", resp.Body) // not two diffs, just one
	})
}

func TestPullCreate_TitleEscape(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "<i>XSS PR</i>")

		// check the redirected URL
		url := test.RedirectURL(resp)
		assert.Regexp(t, "^/user2/repo1/pulls/[0-9]*$", url)

		// Edit title
		req := NewRequest(t, "GET", url)
		resp = session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		editTestTitleURL, exists := htmlDoc.doc.Find(".issue-title-buttons button[data-update-url]").First().Attr("data-update-url")
		assert.True(t, exists, "The template has changed")

		req = NewRequestWithValues(t, "POST", editTestTitleURL, map[string]string{
			"_csrf": htmlDoc.GetCSRF(),
			"title": "<u>XSS PR</u>",
		})
		session.MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", url)
		resp = session.MakeRequest(t, req, http.StatusOK)
		htmlDoc = NewHTMLParser(t, resp.Body)
		titleHTML, err := htmlDoc.doc.Find(".comment-list .timeline-item.event .text b").First().Html()
		assert.NoError(t, err)
		assert.Equal(t, "<strike>&lt;i&gt;XSS PR&lt;/i&gt;</strike>", titleHTML)
		titleHTML, err = htmlDoc.doc.Find(".comment-list .timeline-item.event .text b").Next().Html()
		assert.NoError(t, err)
		assert.Equal(t, "&lt;u&gt;XSS PR&lt;/u&gt;", titleHTML)
	})
}

func testUIDeleteBranch(t *testing.T, session *TestSession, ownerName, repoName, branchName string) {
	relURL := "/" + path.Join(ownerName, repoName, "branches")
	req := NewRequest(t, "GET", relURL)
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	req = NewRequestWithValues(t, "POST", relURL+"/delete", map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"name":  branchName,
	})
	session.MakeRequest(t, req, http.StatusOK)
}

func testDeleteRepository(t *testing.T, session *TestSession, ownerName, repoName string) {
	relURL := "/" + path.Join(ownerName, repoName, "settings")
	req := NewRequest(t, "GET", relURL)
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	req = NewRequestWithValues(t, "POST", relURL+"?action=delete", map[string]string{
		"_csrf":     htmlDoc.GetCSRF(),
		"repo_name": repoName,
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
}

func TestPullBranchDelete(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		defer tests.PrepareTestEnv(t)()

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testCreateBranch(t, session, "user1", "repo1", "branch/master", "master1", http.StatusSeeOther)
		testEditFile(t, session, "user1", "repo1", "master1", "README.md", "Hello, World (Edited)\n")
		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master1", "This is a pull title")

		// check the redirected URL
		url := test.RedirectURL(resp)
		assert.Regexp(t, "^/user2/repo1/pulls/[0-9]*$", url)
		req := NewRequest(t, "GET", url)
		session.MakeRequest(t, req, http.StatusOK)

		// delete head branch and confirm pull page is ok
		testUIDeleteBranch(t, session, "user1", "repo1", "master1")
		req = NewRequest(t, "GET", url)
		session.MakeRequest(t, req, http.StatusOK)

		// delete head repository and confirm pull page is ok
		testDeleteRepository(t, session, "user1", "repo1")
		req = NewRequest(t, "GET", url)
		session.MakeRequest(t, req, http.StatusOK)
	})
}

/*
Setup:
The base repository is: user2/repo1
Fork repository to: user1/repo1
Push extra commit to: user2/repo1, which changes README.md
Create a PR on user1/repo1

Test checks:
Check if pull request can be created from base to the fork repository.
*/
func TestPullCreatePrFromBaseToFork(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		sessionFork := loginUser(t, "user1")
		testRepoFork(t, sessionFork, "user2", "repo1", "user1", "repo1", "")

		// Edit base repository
		sessionBase := loginUser(t, "user2")
		testEditFile(t, sessionBase, "user2", "repo1", "master", "README.md", "Hello, World (Edited)\n")

		// Create a PR
		resp := testPullCreateDirectly(t, sessionFork, "user1", "repo1", "master", "user2", "repo1", "master", "This is a pull title")
		// check the redirected URL
		url := test.RedirectURL(resp)
		assert.Regexp(t, "^/user1/repo1/pulls/[0-9]*$", url)
	})
}

func TestCreateAgitPullWithReadPermission(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		dstPath := t.TempDir()

		u.Path = "user2/repo1.git"
		u.User = url.UserPassword("user4", userPassword)

		t.Run("Clone", doGitClone(dstPath, u))

		t.Run("add commit", doGitAddSomeCommits(dstPath, "master"))

		t.Run("do agit pull create", func(t *testing.T) {
			err := git.NewCommand(git.DefaultContext, "push", "origin", "HEAD:refs/for/master", "-o").AddDynamicArguments("topic=" + "test-topic").Run(&git.RunOpts{Dir: dstPath})
			assert.NoError(t, err)
		})
	})
}

/*
Setup: user2 has repository, user1 forks it
---

1. User2 blocks User1
2. User1 adds changes to fork
3. User1 attempts to create a pull request
4. User1 sees alert that the action is not allowed because of the block
*/
func TestCreatePullWhenBlocked(t *testing.T) {
	RepoOwner := "user2"
	ForkOwner := "user16"
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// Setup
		// User1 forks repo1 from User2
		sessionFork := loginUser(t, ForkOwner)
		testRepoFork(t, sessionFork, RepoOwner, "repo1", ForkOwner, "forkrepo1", "")

		// 1. User2 blocks user1
		// sessionBase := loginUser(t, "user2")
		token := getUserToken(t, RepoOwner, auth_model.AccessTokenScopeWriteUser)

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/user/blocks/%s", ForkOwner)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "PUT", fmt.Sprintf("/api/v1/user/blocks/%s", ForkOwner)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		// 2. User1 adds changes to fork
		testEditFile(t, sessionFork, ForkOwner, "forkrepo1", "master", "README.md", "Hello, World (Edited)\n")

		// 3. User1 attempts to create a pull request
		testPullCreateFailure(t, sessionFork, RepoOwner, "repo1", "master", ForkOwner, "forkrepo1", "master", "This is a pull title")

		// Teardown
		// Unblock user
		req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/user/blocks/%s", ForkOwner)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
	})
}
