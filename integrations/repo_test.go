// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func TestViewRepo(t *testing.T) {
	defer prepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo1")
	MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", "/user3/repo3")
	MakeRequest(t, req, http.StatusNotFound)

	session := loginUser(t, "user1")
	session.MakeRequest(t, req, http.StatusNotFound)
}

func TestViewRepo2(t *testing.T) {
	defer prepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user3/repo3")
	session := loginUser(t, "user2")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestViewRepo3(t *testing.T) {
	defer prepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user3/repo3")
	session := loginUser(t, "user4")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestViewRepo1CloneLinkAnonymous(t *testing.T) {
	defer prepareTestEnv(t)()

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
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo1")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("#repo-clone-https").Attr("data-link")
	assert.True(t, exists, "The template has changed")
	assert.Equal(t, setting.AppURL+"user2/repo1.git", link)
	link, exists = htmlDoc.doc.Find("#repo-clone-ssh").Attr("data-link")
	assert.True(t, exists, "The template has changed")
	sshURL := fmt.Sprintf("ssh://%s@%s:%d/user2/repo1.git", setting.SSH.BuiltinServerUser, setting.SSH.Domain, setting.SSH.Port)
	assert.Equal(t, sshURL, link)
}

func TestViewRepoWithSymlinks(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo20.git")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	files := htmlDoc.doc.Find("#repo-files-table > TBODY > TR > TD.name > SPAN")
	items := files.Map(func(i int, s *goquery.Selection) string {
		cls, _ := s.Find("SPAN").Attr("class")
		file := strings.Trim(s.Find("A").Text(), " \t\n")
		return fmt.Sprintf("%s: %s", file, cls)
	})
	assert.Equal(t, len(items), 5)
	assert.Equal(t, items[0], "a: octicon octicon-file-directory")
	assert.Equal(t, items[1], "link_b: octicon octicon-file-symlink-directory")
	assert.Equal(t, items[2], "link_d: octicon octicon-file-symlink-file")
	assert.Equal(t, items[3], "link_hi: octicon octicon-file-symlink-file")
	assert.Equal(t, items[4], "link_link: octicon octicon-file-symlink-file")
}

// TestViewAsRepoAdmin tests PR #2167
func TestViewAsRepoAdmin(t *testing.T) {
	for user, expectedNoDescription := range map[string]bool{
		"user2": true,
		"user4": false,
	} {
		defer prepareTestEnv(t)()

		session := loginUser(t, user)

		req := NewRequest(t, "GET", "/user2/repo1.git")
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)
		noDescription := htmlDoc.doc.Find("#repo-desc").Children()

		assert.Equal(t, expectedNoDescription, noDescription.HasClass("no-description"))
	}
}
