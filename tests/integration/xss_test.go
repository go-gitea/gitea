// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
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
	const fullName = `name & <script class="evil">alert('Oh no!');</script>`

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
	assert.EqualValues(t, fullName,
		htmlDoc.doc.Find("div.content").Find(".header.text.center").Text(),
	)
}

func TestXSSWikiLastCommitInfo(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// Prepare the environment.
		dstPath := t.TempDir()
		r := fmt.Sprintf("%suser2/repo1.wiki.git", u.String())
		u, err := url.Parse(r)
		assert.NoError(t, err)
		u.User = url.UserPassword("user2", userPassword)
		assert.NoError(t, git.CloneWithArgs(context.Background(), git.AllowLFSFiltersArgs(), u.String(), dstPath, git.CloneRepoOptions{}))

		// Use go-git here, because using git wouldn't work, it has code to remove
		// `<`, `>` and `\n` in user names. Even though this is permitted and
		// wouldn't result in a error by a Git server.
		gitRepo, err := gogit.PlainOpen(dstPath)
		if err != nil {
			panic(err)
		}

		w, err := gitRepo.Worktree()
		if err != nil {
			panic(err)
		}

		filename := filepath.Join(dstPath, "Home.md")
		err = os.WriteFile(filename, []byte("Oh, a XSS attack?"), 0o644)
		if err != nil {
			panic(err)
		}

		_, err = w.Add("Home.md")
		if err != nil {
			panic(err)
		}

		_, err = w.Commit("Yay XSS", &gogit.CommitOptions{
			Author: &object.Signature{
				Name:  `Gusted <script class="evil">alert('Oh no!');</script>`,
				Email: "valid@example.org",
				When:  time.Date(2024, time.January, 31, 0, 0, 0, 0, time.UTC),
			},
		})

		// Push.
		_, _, err = git.NewCommand(git.DefaultContext, "push").AddArguments(git.ToTrustedCmdArgs([]string{"origin", "master"})...).RunStdString(&git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)

		// Check on page view.
		t.Run("Page view", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, http.MethodGet, "/user2/repo1/wiki/Home")
			resp := MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)

			htmlDoc.AssertElement(t, "script.evil", false)
			assert.EqualValues(t, `Gusted <script class="evil">alert('Oh no!');</script> edited this page 2024-01-31 00:00:00 +00:00`, strings.TrimSpace(htmlDoc.Find(".ui.sub.header").Text()))
		})

		// Check on revisions page.
		t.Run("Revision page", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, http.MethodGet, "/user2/repo1/wiki/Home?action=_revision")
			resp := MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)

			htmlDoc.AssertElement(t, "script.evil", false)
			assert.EqualValues(t, `Gusted <script class="evil">alert('Oh no!');</script> edited this page 2024-01-31 00:00:00 +00:00`, strings.TrimSpace(htmlDoc.Find(".ui.sub.header").Text()))
		})
	})
}
