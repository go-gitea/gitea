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

func testPullMerge(t *testing.T, session *TestSession, user, repo, pullnum string) *TestResponse {
	req := NewRequest(t, "GET", path.Join(user, repo, "pulls", pullnum))
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	// Click the little green button to craete a pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("form.ui.form>button.ui.green.button").Parent().Attr("action")
	assert.True(t, exists, "The template has changed")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
	})
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusFound, resp.HeaderCode)

	return resp
}

func TestPullMerge(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user1")
	testRepoFork(t, session)
	testEditFile(t, session, "user1", "repo1", "master", "README.md")

	resp := testPullCreate(t, session, "user1", "repo1", "master")
	redirectedURL := resp.Headers["Location"]
	assert.NotEmpty(t, redirectedURL, "Redirected URL is not found")

	elem := strings.Split(redirectedURL[0], "/")
	assert.EqualValues(t, "pulls", elem[3])
	testPullMerge(t, session, elem[1], elem[2], elem[4])
}
