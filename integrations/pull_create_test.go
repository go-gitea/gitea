// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testPullCreate(t *testing.T, session *TestSession, user, repo, branch, title string) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", path.Join(user, repo))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Click the PR button to create a pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("#new-pull-request").Parent().Attr("href")
	assert.True(t, exists, "The template has changed")
	if branch != "master" {
		link = strings.Replace(link, ":master", ":"+branch, 1)
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
	resp = session.MakeRequest(t, req, http.StatusFound)

	return resp
}

func TestPullCreate(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		resp := testPullCreate(t, session, "user1", "repo1", "master", "This is a pull title")

		// check the redirected URL
		url := resp.HeaderMap.Get("Location")
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
		assert.Regexp(t, `Subject: \[PATCH\] Update 'README.md'`, resp.Body)
		assert.NotRegexp(t, "diff.*diff", resp.Body) // not two diffs, just one
	})
}

func TestPullCreate_TitleEscape(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		resp := testPullCreate(t, session, "user1", "repo1", "master", "<i>XSS PR</i>")

		// check the redirected URL
		url := resp.HeaderMap.Get("Location")
		assert.Regexp(t, "^/user2/repo1/pulls/[0-9]*$", url)

		// Edit title
		req := NewRequest(t, "GET", url)
		resp = session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		editTestTitleURL, exists := htmlDoc.doc.Find("#save-edit-title").First().Attr("data-update-url")
		assert.True(t, exists, "The template has changed")

		req = NewRequestWithValues(t, "POST", editTestTitleURL, map[string]string{
			"_csrf": htmlDoc.GetCSRF(),
			"title": "<u>XSS PR</u>",
		})
		session.MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", url)
		resp = session.MakeRequest(t, req, http.StatusOK)
		htmlDoc = NewHTMLParser(t, resp.Body)
		titleHTML, err := htmlDoc.doc.Find(".comments .event .text b").First().Html()
		assert.NoError(t, err)
		assert.Equal(t, "<strike>&lt;i&gt;XSS PR&lt;/i&gt;</strike>", titleHTML)
		titleHTML, err = htmlDoc.doc.Find(".comments .event .text b").Next().Html()
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
		"_csrf": getCsrf(t, htmlDoc.doc),
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
		"_csrf":     getCsrf(t, htmlDoc.doc),
		"repo_name": repoName,
	})
	session.MakeRequest(t, req, http.StatusFound)
}

func TestPullBranchDelete(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		defer prepareTestEnv(t)()

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testCreateBranch(t, session, "user1", "repo1", "branch/master", "master1", http.StatusFound)
		testEditFile(t, session, "user1", "repo1", "master1", "README.md", "Hello, World (Edited)\n")
		resp := testPullCreate(t, session, "user1", "repo1", "master1", "This is a pull title")

		// check the redirected URL
		url := resp.HeaderMap.Get("Location")
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
