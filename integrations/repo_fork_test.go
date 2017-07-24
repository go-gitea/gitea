// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testRepoFork(t *testing.T, session *TestSession) *TestResponse {
	// Step0: check the existence of the to-fork repo
	req := NewRequest(t, "GET", "/user1/repo1")
	resp := session.MakeRequest(t, req, http.StatusNotFound)

	// Step1: go to the main page of repo
	req = NewRequest(t, "GET", "/user2/repo1")
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Step2: click the fork button
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("a.ui.button[href^=\"/repo/fork/\"]").Attr("href")
	assert.True(t, exists, "The template has changed")
	req = NewRequest(t, "GET", link)
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Step3: fill the form of the forking
	htmlDoc = NewHTMLParser(t, resp.Body)
	link, exists = htmlDoc.doc.Find("form.ui.form[action^=\"/repo/fork/\"]").Attr("action")
	assert.True(t, exists, "The template has changed")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf":     htmlDoc.GetCSRF(),
		"uid":       "1",
		"repo_name": "repo1",
	})
	resp = session.MakeRequest(t, req, http.StatusFound)

	// Step4: check the existence of the forked repo
	req = NewRequest(t, "GET", "/user1/repo1")
	resp = session.MakeRequest(t, req, http.StatusOK)

	return resp
}

func TestRepoFork(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user1")
	testRepoFork(t, session)
}
