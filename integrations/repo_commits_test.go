// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"net/http"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepoCommits(t *testing.T) {
	prepareTestEnv(t)

	session := loginUser(t, "user2", "password")

	// Request repository commits page
	req := NewRequest(t, "GET", "/user2/repo1/commits/master")
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	doc, err := NewHtmlParser(resp.Body)
	assert.NoError(t, err)
	commitURL, exists := doc.doc.Find("#commits-table tbody tr td.sha a").Attr("href")
	assert.True(t, exists)
	assert.NotEmpty(t, commitURL)
}

func doTestRepoCommitWithStatus(t *testing.T, state string, classes ...string) {
	prepareTestEnv(t)

	session := loginUser(t, "user2", "password")

	// Request repository commits page
	req := NewRequest(t, "GET", "/user2/repo1/commits/master")
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	doc, err := NewHtmlParser(resp.Body)
	assert.NoError(t, err)
	// Get first commit URL
	commitURL, exists := doc.doc.Find("#commits-table tbody tr td.sha a").Attr("href")
	assert.True(t, exists)
	assert.NotEmpty(t, commitURL)

	// Call API to add status for commit
	req = NewRequestBody(t, "POST", "/api/v1/repos/user2/repo1/statuses/"+path.Base(commitURL),
		bytes.NewBufferString("{\"state\":\""+state+"\", \"target_url\": \"http://test.ci/\", \"description\": \"\", \"context\": \"testci\"}"))

	req.Header.Add("Content-Type", "application/json")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusCreated, resp.HeaderCode)

	req = NewRequest(t, "GET", "/user2/repo1/commits/master")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	doc, err = NewHtmlParser(resp.Body)
	assert.NoError(t, err)
	// Check if commit status is displayed in message column
	sel := doc.doc.Find("#commits-table tbody tr td.message i.commit-status")
	assert.Equal(t, sel.Length(), 1)
	for _, class := range classes {
		assert.True(t, sel.HasClass(class))
	}
}

func TestRepoCommitsWithStatusPending(t *testing.T) {
	doTestRepoCommitWithStatus(t, "pending", "circle", "yellow")
}

func TestRepoCommitsWithStatusSuccess(t *testing.T) {
	doTestRepoCommitWithStatus(t, "success", "check", "green")
}

func TestRepoCommitsWithStatusError(t *testing.T) {
	doTestRepoCommitWithStatus(t, "error", "warning", "red")
}

func TestRepoCommitsWithStatusFailure(t *testing.T) {
	doTestRepoCommitWithStatus(t, "failure", "remove", "red")
}

func TestRepoCommitsWithStatusWarning(t *testing.T) {
	doTestRepoCommitWithStatus(t, "warning", "warning", "sign", "yellow")
}
