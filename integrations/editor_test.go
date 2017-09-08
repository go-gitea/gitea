// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateFile(t *testing.T) {
	prepareTestEnv(t)

	session := loginUser(t, "user2")

	// Request editor page
	req := NewRequest(t, "GET", "/user2/repo1/_new/master/")
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
	resp = session.MakeRequest(t, req, http.StatusFound)
}

func TestCreateFileOnProtectedBranch(t *testing.T) {
	prepareTestEnv(t)

	session := loginUser(t, "user2")

	csrf := GetCSRF(t, session, "/user2/repo1/settings/branches")
	// Change master branch to protected
	req := NewRequestWithValues(t, "POST", "/user2/repo1/settings/branches/master", map[string]string{
		"_csrf":     csrf,
		"protected": "on",
	})
	resp := session.MakeRequest(t, req, http.StatusFound)
	// Check if master branch has been locked successfully
	flashCookie := session.GetCookie("macaron_flash")
	assert.NotNil(t, flashCookie)
	assert.EqualValues(t, "success%3DBranch%2Bmaster%2Bprotect%2Boptions%2Bchanged%2Bsuccessfully.", flashCookie.Value)

	// Request editor page
	req = NewRequest(t, "GET", "/user2/repo1/_new/master/")
	resp = session.MakeRequest(t, req, http.StatusOK)

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
	assert.Contains(t, string(resp.Body), "Can not commit to protected branch &#39;master&#39;.")

	// remove the protected branch
	csrf = GetCSRF(t, session, "/user2/repo1/settings/branches")
	// Change master branch to protected
	req = NewRequestWithValues(t, "POST", "/user2/repo1/settings/branches/master", map[string]string{
		"_csrf":     csrf,
		"protected": "off",
	})
	resp = session.MakeRequest(t, req, http.StatusFound)
	// Check if master branch has been locked successfully
	flashCookie = session.GetCookie("macaron_flash")
	assert.NotNil(t, flashCookie)
	assert.EqualValues(t, "success%3DBranch%2Bmaster%2Bprotect%2Boptions%2Bremoved%2Bsuccessfully", flashCookie.Value)

}

func testEditFile(t *testing.T, session *TestSession, user, repo, branch, filePath string) *TestResponse {

	newContent := "Hello, World (Edited)\n"

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
	resp = session.MakeRequest(t, req, http.StatusFound)

	// Verify the change
	req = NewRequest(t, "GET", path.Join(user, repo, "raw", branch, filePath))
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.EqualValues(t, newContent, string(resp.Body))

	return resp
}

func testEditFileToNewBranch(t *testing.T, session *TestSession, user, repo, branch, targetBranch, filePath string) *TestResponse {

	newContent := "Hello, World (Edited)\n"

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
	resp = session.MakeRequest(t, req, http.StatusFound)

	// Verify the change
	req = NewRequest(t, "GET", path.Join(user, repo, "raw", targetBranch, filePath))
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.EqualValues(t, newContent, string(resp.Body))

	return resp
}

func TestEditFile(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user2")
	testEditFile(t, session, "user2", "repo1", "master", "README.md")
}

func TestEditFileToNewBranch(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user2")
	testEditFileToNewBranch(t, session, "user2", "repo1", "master", "feature/test", "README.md")
}
