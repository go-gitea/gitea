// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateFile(t *testing.T) {
	prepareTestEnv(t)

	session := loginUser(t, "user2", "password")

	// Request editor page
	req, err := http.NewRequest("GET", "/user2/repo1/_new/master/", nil)
	assert.NoError(t, err)
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	doc, err := NewHtmlParser(resp.Body)
	assert.NoError(t, err)
	lastCommit := doc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Save new file to master branch
	req, err = http.NewRequest("POST", "/user2/repo1/_new/master/",
		bytes.NewBufferString(url.Values{
			"_csrf":         []string{doc.GetInputValueByName("_csrf")},
			"last_commit":   []string{lastCommit},
			"tree_path":     []string{"test.txt"},
			"content":       []string{"Content"},
			"commit_choice": []string{"direct"},
		}.Encode()),
	)
	assert.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusFound, resp.HeaderCode)
}

func TestCreateFileOnProtectedBranch(t *testing.T) {
	prepareTestEnv(t)

	session := loginUser(t, "user2", "password")

	// Open repository branch settings
	req, err := http.NewRequest("GET", "/user2/repo1/settings/branches", nil)
	assert.NoError(t, err)
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	doc, err := NewHtmlParser(resp.Body)
	assert.NoError(t, err)

	// Change master branch to protected
	req, err = http.NewRequest("POST", "/user2/repo1/settings/branches?action=protected_branch",
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
	req, err = http.NewRequest("GET", "/user2/repo1/_new/master/", nil)
	assert.NoError(t, err)
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	doc, err = NewHtmlParser(resp.Body)
	assert.NoError(t, err)
	lastCommit := doc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Save new file to master branch
	req, err = http.NewRequest("POST", "/user2/repo1/_new/master/",
		bytes.NewBufferString(url.Values{
			"_csrf":         []string{doc.GetInputValueByName("_csrf")},
			"last_commit":   []string{lastCommit},
			"tree_path":     []string{"test.txt"},
			"content":       []string{"Content"},
			"commit_choice": []string{"direct"},
		}.Encode()),
	)
	assert.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
	// Check body for error message
	assert.Contains(t, string(resp.Body), "Can not commit to protected branch &#39;master&#39;.")
}
