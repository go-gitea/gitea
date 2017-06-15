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

func testPullCreate(t *testing.T, session *TestSession, user, repo, branch string) {
	req := NewRequest(t, "GET", path.Join(user, repo))
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	// Click the little green button to create a pull
	htmlDoc, err := NewHtmlParser(resp.Body)
	assert.NoError(t, err)
	link, exists := htmlDoc.doc.Find("button.ui.green.small.button").Parent().Attr("href")
	assert.True(t, exists, "The template has changed")

	req = NewRequest(t, "GET", link)
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	// Submit the form for creating the pull
	htmlDoc, err = NewHtmlParser(resp.Body)
	assert.NoError(t, err)
	link, exists = htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")
	req = NewRequestBody(t, "POST", link,
		bytes.NewBufferString(url.Values{
			"_csrf": []string{htmlDoc.GetInputValueByName("_csrf")},
			"title": []string{"This is a pull title"},
		}.Encode()),
	)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusFound, resp.HeaderCode)

	//TODO check the redirected URL
}

func TestPullCreate(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user1", "password")
	testRepoFork(t, session)
	testEditFile(t, session, "user1", "repo1", "master", "README.md")
	testPullCreate(t, session, "user1", "repo1", "master")
}
