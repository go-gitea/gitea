// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestViewRepo(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user2/repo1")
	MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", "/user3/repo3")
	MakeRequest(t, req, http.StatusNotFound)

	session := loginUser(t, "user1")
	session.MakeRequest(t, req, http.StatusNotFound)
}

func TestViewRepo2(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user3/repo3")
	session := loginUser(t, "user2")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestViewRepo3(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user3/repo3")
	session := loginUser(t, "user4")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestViewRepo1CloneLinkAnonymous(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user2/repo1")
	resp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("#repo-clone-https").Attr("data-link")
	assert.True(t, exists, "The template has changed")
	assert.Equal(t, setting.AppURL+"user2/repo1.git", link)
	_, exists = htmlDoc.doc.Find("#repo-clone-ssh").Attr("data-link")
	assert.False(t, exists)
}

func TestViewRepo1CloneLinkAuthorized(t *testing.T) {
	prepareTestEnv(t)

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo1")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("#repo-clone-https").Attr("data-link")
	assert.True(t, exists, "The template has changed")
	assert.Equal(t, setting.AppURL+"user2/repo1.git", link)
	link, exists = htmlDoc.doc.Find("#repo-clone-ssh").Attr("data-link")
	assert.True(t, exists, "The template has changed")
	sshURL := fmt.Sprintf("%s@%s:user2/repo1.git", setting.RunUser, setting.SSH.Domain)
	assert.Equal(t, sshURL, link)
}
