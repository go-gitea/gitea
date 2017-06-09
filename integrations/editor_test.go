// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"net/http"
	"net/url"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateFile(t *testing.T) {
	prepareTestEnv(t)

	session := loginUser(t, "user2", "password")

	// Request editor page
	req := NewRequest(t, "GET", "/user2/repo1/_new/master/")
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	doc, err := NewHtmlParser(resp.Body)
	assert.NoError(t, err)
	lastCommit := doc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Save new file to master branch
	req = NewRequestBody(t, "POST", "/user2/repo1/_new/master/",
		bytes.NewBufferString(url.Values{
			"_csrf":         []string{doc.GetInputValueByName("_csrf")},
			"last_commit":   []string{lastCommit},
			"tree_path":     []string{"test.txt"},
			"content":       []string{"Content"},
			"commit_choice": []string{"direct"},
		}.Encode()),
	)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusFound, resp.HeaderCode)
}

func TestCreateFileOnProtectedBranch(t *testing.T) {
	prepareTestEnv(t)

	session := loginUser(t, "user2", "password")

	// Open repository branch settings
	req := NewRequest(t, "GET", "/user2/repo1/settings/branches")
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	doc, err := NewHtmlParser(resp.Body)
	assert.NoError(t, err)

	// Change master branch to protected
	req = NewRequestBody(t, "POST", "/user2/repo1/settings/branches?action=protected_branch",
		bytes.NewBufferString(url.Values{
			"_csrf":      []string{doc.GetInputValueByName("_csrf")},
			"branchName": []string{"master"},
			"canPush":    []string{"true"},
		}.Encode()),
	)
	assert.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
	// Check if master branch has been locked successfully
	flashCookie := session.GetCookie("macaron_flash")
	assert.NotNil(t, flashCookie)
	assert.EqualValues(t, flashCookie.Value, "success%3Dmaster%2BLocked%2Bsuccessfully")

	// Request editor page
	req = NewRequest(t, "GET", "/user2/repo1/_new/master/")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	doc, err = NewHtmlParser(resp.Body)
	assert.NoError(t, err)
	lastCommit := doc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Save new file to master branch
	req = NewRequestBody(t, "POST", "/user2/repo1/_new/master/",
		bytes.NewBufferString(url.Values{
			"_csrf":         []string{doc.GetInputValueByName("_csrf")},
			"last_commit":   []string{lastCommit},
			"tree_path":     []string{"test.txt"},
			"content":       []string{"Content"},
			"commit_choice": []string{"direct"},
		}.Encode()),
	)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
	// Check body for error message
	assert.Contains(t, string(resp.Body), "Can not commit to protected branch &#39;master&#39;.")
}

func testEditFile(t *testing.T, session *TestSession, user, repo, branch, filePath string) {

	newContent := "Hello, World (Edited)\n"

	// Get to the 'edit this file' page
	req := NewRequest(t, "GET", path.Join(user, repo, "_edit", branch, filePath))
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	htmlDoc, err := NewHtmlParser(resp.Body)
	assert.NoError(t, err)
	lastCommit := htmlDoc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Submit the edits
	req = NewRequestBody(t, "POST", path.Join(user, repo, "_edit", branch, filePath),
		bytes.NewBufferString(url.Values{
			"_csrf":         []string{htmlDoc.GetInputValueByName("_csrf")},
			"last_commit":   []string{lastCommit},
			"tree_path":     []string{filePath},
			"content":       []string{newContent},
			"commit_choice": []string{"direct"},
		}.Encode()),
	)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusFound, resp.HeaderCode)

	// Verify the change
	req = NewRequest(t, "GET", path.Join(user, repo, "raw", branch, filePath))
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
	assert.EqualValues(t, newContent, string(resp.Body))
}

func TestEditFile(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user2", "password")
	testEditFile(t, session, "user2", "repo1", "master", "README.md")
}
