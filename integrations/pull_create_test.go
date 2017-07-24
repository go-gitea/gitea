// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testPullCreate(t *testing.T, session *TestSession, user, repo, branch string) *TestResponse {
	req := NewRequest(t, "GET", path.Join(user, repo))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Click the little green button to create a pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("button.ui.green.small.button").Parent().Attr("href")
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
		"title": "This is a pull title",
	})
	resp = session.MakeRequest(t, req, http.StatusFound)

	//TODO check the redirected URL

	return resp
}

func TestPullCreate(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user1")
	testRepoFork(t, session)
	testEditFile(t, session, "user1", "repo1", "master", "README.md")
	testPullCreate(t, session, "user1", "repo1", "master")
}
