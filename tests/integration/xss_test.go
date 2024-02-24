// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/tests"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
)

func TestXSSUserFullName(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	const fullName = `name & <script class="evil">alert('xss');</script>`

	session := loginUser(t, user.Name)
	req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
		"_csrf":     GetCSRF(t, session, "/user/settings"),
		"name":      user.Name,
		"full_name": fullName,
		"email":     user.Email,
		"language":  "en-US",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	req = NewRequestf(t, "GET", "/%s", user.Name)
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.EqualValues(t, 0, htmlDoc.doc.Find("script.evil").Length())
	htmlCode, err := htmlDoc.doc.Find("div.content").Find(".header.text.center").Html()
	assert.NoError(t, err)
	assert.EqualValues(t, html.EscapeString(fullName), htmlCode)
}

func TestXSSWikiLastCommitInfo(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		dstPath := t.TempDir()
		cloneWikiURL, err := url.Parse(u.String() + "user2/repo1.wiki.git")
		assert.NoError(t, err)
		cloneWikiURL.User = url.UserPassword("user2", userPassword)
		assert.NoError(t, git.CloneWithArgs(context.Background(), git.AllowLFSFiltersArgs(), cloneWikiURL.String(), dstPath, git.CloneRepoOptions{}))

		// Use go-git here, because using git wouldn't work, it has code to remove
		// `<`, `>` and `\n` in user names. Even though this is permitted and
		// wouldn't result in a error by a Git server.
		gitRepo, err := gogit.PlainOpen(dstPath)
		if !assert.NoError(t, err) {
			return
		}
		w, err := gitRepo.Worktree()
		if !assert.NoError(t, err) {
			return
		}

		filename := filepath.Join(dstPath, "Home.md")
		err = os.WriteFile(filename, []byte("dummy content"), 0o644)
		if !assert.NoError(t, err) {
			return
		}

		_, err = w.Add("Home.md")
		if !assert.NoError(t, err) {
			return
		}

		_, err = w.Commit("dummy message", &gogit.CommitOptions{
			Author: &object.Signature{
				Name:  `foo<script class="evil">alert('xss');</script>bar`,
				Email: "valid@example.org",
				When:  time.Date(2001, time.January, 31, 0, 0, 0, 0, time.UTC),
			},
		})
		if !assert.NoError(t, err) {
			return
		}

		// Push.
		_, _, err = git.NewCommand(git.DefaultContext, "push").AddArguments("origin", "master").RunStdString(&git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)

		// Check on page view.
		t.Run("Page view", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, http.MethodGet, "/user2/repo1/wiki/Home")
			resp := MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)

			htmlDoc.AssertElement(t, "script.evil", false)
			htmlCode, err := htmlDoc.Find(".ui.sub.header").Html()
			assert.NoError(t, err)
			assert.EqualValues(t, `foo&lt;script class=&#34;evil&#34;&gt;alert(&#39;xss&#39;);&lt;/script&gt;bar edited this page <relative-time class="time-since" prefix="" tense="past" datetime="2001-01-31T00:00:00Z" data-tooltip-content="" data-tooltip-interactive="true">2001-01-31 00:00:00 +00:00</relative-time>`, strings.TrimSpace(htmlCode))
		})

		// Check on revisions page.
		t.Run("Revision page", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, http.MethodGet, "/user2/repo1/wiki/Home?action=_revision")
			resp := MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)

			htmlDoc.AssertElement(t, "script.evil", false)
			htmlCode, err := htmlDoc.Find(".ui.sub.header").Html()
			assert.NoError(t, err)
			assert.EqualValues(t, `foo&lt;script class=&#34;evil&#34;&gt;alert(&#39;xss&#39;);&lt;/script&gt;bar edited this page <relative-time class="time-since" prefix="" tense="past" datetime="2001-01-31T00:00:00Z" data-tooltip-content="" data-tooltip-interactive="true">2001-01-31 00:00:00 +00:00</relative-time>`, strings.TrimSpace(htmlCode))
		})
	})
}
