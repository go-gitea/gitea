// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"testing"

	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
)

func TestCreateFile(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		testCreateFile(t, session, "user2", "repo1", "master", "test.txt", "Content")
	})
}

func testCreateFile(t *testing.T, session *TestSession, user, repo, branch, filePath, content string) *httptest.ResponseRecorder {
	// Request editor page
	newURL := fmt.Sprintf("/%s/%s/_new/%s/", user, repo, branch)
	req := NewRequest(t, "GET", newURL)
	resp := session.MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	lastCommit := doc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Save new file to master branch
	req = NewRequestWithValues(t, "POST", newURL, map[string]string{
		"_csrf":         doc.GetCSRF(),
		"last_commit":   lastCommit,
		"tree_path":     filePath,
		"content":       content,
		"commit_choice": "direct",
	})
	return session.MakeRequest(t, req, http.StatusSeeOther)
}

func TestCreateFileOnProtectedBranch(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")

		csrf := GetUserCSRFToken(t, session)
		// Change master branch to protected
		req := NewRequestWithValues(t, "POST", "/user2/repo1/settings/branches/edit", map[string]string{
			"_csrf":       csrf,
			"rule_name":   "master",
			"enable_push": "true",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)
		// Check if master branch has been locked successfully
		flashMsg := session.GetCookieFlashMessage()
		assert.EqualValues(t, `Branch protection for rule "master" has been updated.`, flashMsg.SuccessMsg)

		// Request editor page
		req = NewRequest(t, "GET", "/user2/repo1/_new/master/")
		resp := session.MakeRequest(t, req, http.StatusOK)

		doc := NewHTMLParser(t, resp.Body)
		lastCommit := doc.GetInputValueByName("last_commit")
		assert.NotEmpty(t, lastCommit)

		// Save new file to master branch
		req = NewRequestWithValues(t, "POST", "/user2/repo1/_new/master/", map[string]string{
			"_csrf":         doc.GetCSRF(),
			"last_commit":   lastCommit,
			"tree_path":     "test.txt",
			"content":       "Content",
			"commit_choice": "direct",
		})

		resp = session.MakeRequest(t, req, http.StatusOK)
		// Check body for error message
		assert.Contains(t, resp.Body.String(), "Cannot commit to protected branch &#34;master&#34;.")

		// remove the protected branch
		csrf = GetUserCSRFToken(t, session)

		// Change master branch to protected
		req = NewRequestWithValues(t, "POST", "/user2/repo1/settings/branches/1/delete", map[string]string{
			"_csrf": csrf,
		})

		resp = session.MakeRequest(t, req, http.StatusOK)

		res := make(map[string]string)
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.EqualValues(t, "/user2/repo1/settings/branches", res["redirect"])

		// Check if master branch has been locked successfully
		flashMsg = session.GetCookieFlashMessage()
		assert.EqualValues(t, `Removing branch protection rule "1" failed.`, flashMsg.ErrorMsg)
	})
}

func testEditFile(t *testing.T, session *TestSession, user, repo, branch, filePath, newContent string) *httptest.ResponseRecorder {
	// Get to the 'edit this file' page
	req := NewRequest(t, "GET", path.Join(user, repo, "_edit", branch, filePath))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	lastCommit := htmlDoc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Submit the edits
	req = NewRequestWithValues(t, "POST", path.Join(user, repo, "_edit", branch, filePath),
		map[string]string{
			"_csrf":         htmlDoc.GetCSRF(),
			"last_commit":   lastCommit,
			"tree_path":     filePath,
			"content":       newContent,
			"commit_choice": "direct",
		},
	)
	session.MakeRequest(t, req, http.StatusSeeOther)

	// Verify the change
	req = NewRequest(t, "GET", path.Join(user, repo, "raw/branch", branch, filePath))
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.EqualValues(t, newContent, resp.Body.String())

	return resp
}

func testEditFileToNewBranch(t *testing.T, session *TestSession, user, repo, branch, targetBranch, filePath, newContent string) *httptest.ResponseRecorder {
	// Get to the 'edit this file' page
	req := NewRequest(t, "GET", path.Join(user, repo, "_edit", branch, filePath))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	lastCommit := htmlDoc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Submit the edits
	req = NewRequestWithValues(t, "POST", path.Join(user, repo, "_edit", branch, filePath),
		map[string]string{
			"_csrf":           htmlDoc.GetCSRF(),
			"last_commit":     lastCommit,
			"tree_path":       filePath,
			"content":         newContent,
			"commit_choice":   "commit-to-new-branch",
			"new_branch_name": targetBranch,
		},
	)
	session.MakeRequest(t, req, http.StatusSeeOther)

	// Verify the change
	req = NewRequest(t, "GET", path.Join(user, repo, "raw/branch", targetBranch, filePath))
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.EqualValues(t, newContent, resp.Body.String())

	return resp
}

func TestEditFile(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		testEditFile(t, session, "user2", "repo1", "master", "README.md", "Hello, World (Edited)\n")
	})
}

func TestEditFileToNewBranch(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		testEditFileToNewBranch(t, session, "user2", "repo1", "master", "feature/test", "README.md", "Hello, World (Edited)\n")
	})
}
