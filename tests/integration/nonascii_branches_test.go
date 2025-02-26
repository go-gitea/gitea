// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"testing"

	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/require"
)

func setDefaultBranch(t *testing.T, session *TestSession, user, repo, branch string) {
	location := path.Join("/", user, repo, "settings/branches")
	csrf := GetUserCSRFToken(t, session)
	req := NewRequestWithValues(t, "POST", location, map[string]string{
		"_csrf":  csrf,
		"action": "default_branch",
		"branch": branch,
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
}

func TestNonAsciiBranches(t *testing.T) {
	testRedirects := []struct {
		from   string
		to     string
		status int
	}{
		// Branches
		{
			from:   "master",
			to:     "branch/master",
			status: http.StatusOK,
		},
		{
			from:   "master/README.md",
			to:     "branch/master/README.md",
			status: http.StatusOK,
		},
		{
			from:   "master/badfile",
			to:     "branch/master/badfile",
			status: http.StatusNotFound, // it does not exist
		},
		{
			from:   "ГлавнаяВетка",
			to:     "branch/%d0%93%d0%bb%d0%b0%d0%b2%d0%bd%d0%b0%d1%8f%d0%92%d0%b5%d1%82%d0%ba%d0%b0",
			status: http.StatusOK,
		},
		{
			from:   "а/б/в",
			to:     "branch/%d0%b0/%d0%b1/%d0%b2",
			status: http.StatusOK,
		},
		{
			from:   "Grüßen/README.md",
			to:     "branch/Gr%c3%bc%c3%9fen/README.md",
			status: http.StatusOK,
		},
		{
			from:   "Plus+Is+Not+Space",
			to:     "branch/Plus+Is+Not+Space",
			status: http.StatusOK,
		},
		{
			from:   "Plus+Is+Not+Space/Файл.md",
			to:     "branch/Plus+Is+Not+Space/%d0%a4%d0%b0%d0%b9%d0%bb.md",
			status: http.StatusOK,
		},
		{
			from:   "Plus+Is+Not+Space/and+it+is+valid.md",
			to:     "branch/Plus+Is+Not+Space/and+it+is+valid.md",
			status: http.StatusOK,
		},
		{
			from:   "ブランチ",
			to:     "branch/%e3%83%96%e3%83%a9%e3%83%b3%e3%83%81",
			status: http.StatusOK,
		},

		// Tags
		{
			from:   "Тэг",
			to:     "tag/%d0%a2%d1%8d%d0%b3",
			status: http.StatusOK,
		},
		{
			from:   "Ё/人",
			to:     "tag/%d0%81/%e4%ba%ba",
			status: http.StatusOK,
		},
		{
			from:   "タグ",
			to:     "tag/%e3%82%bf%e3%82%b0",
			status: http.StatusOK,
		},
		{
			from:   "タグ/ファイル.md",
			to:     "tag/%e3%82%bf%e3%82%b0/%e3%83%95%e3%82%a1%e3%82%a4%e3%83%ab.md",
			status: http.StatusOK,
		},

		// Files
		{
			from:   "README.md",
			to:     "branch/Plus+Is+Not+Space/README.md",
			status: http.StatusOK,
		},
		{
			from:   "Файл.md",
			to:     "branch/Plus+Is+Not+Space/%d0%a4%d0%b0%d0%b9%d0%bb.md",
			status: http.StatusOK,
		},
		{
			from:   "ファイル.md",
			to:     "branch/Plus+Is+Not+Space/%e3%83%95%e3%82%a1%e3%82%a4%e3%83%ab.md",
			status: http.StatusNotFound, // it's not on default branch
		},

		// Same but url-encoded (few tests)
		{
			from:   "%E3%83%96%E3%83%A9%E3%83%B3%E3%83%81",
			to:     "branch/%E3%83%96%E3%83%A9%E3%83%B3%E3%83%81",
			status: http.StatusOK,
		},
		{
			from:   "%E3%82%BF%E3%82%b0",
			to:     "tag/%E3%82%BF%E3%82%b0",
			status: http.StatusOK,
		},
		{
			from:   "%D0%A4%D0%B0%D0%B9%D0%BB.md",
			to:     "branch/Plus+Is+Not+Space/%D0%A4%D0%B0%D0%B9%D0%BB.md",
			status: http.StatusOK,
		},
		{
			from:   "%D0%81%2F%E4%BA%BA",
			to:     "tag/%D0%81%2F%E4%BA%BA",
			status: http.StatusOK,
		},
		{
			from:   "Ё%2F%E4%BA%BA",
			to:     "tag/%d0%81%2F%E4%BA%BA",
			status: http.StatusOK,
		},
		{
			from:   "Plus+Is+Not+Space/%25%252525mightnotplaywell",
			to:     "branch/Plus+Is+Not+Space/%25%252525mightnotplaywell",
			status: http.StatusOK,
		},
		{
			from:   "Plus+Is+Not+Space/%25253Fisnotaquestion%25253F",
			to:     "branch/Plus+Is+Not+Space/%25253Fisnotaquestion%25253F",
			status: http.StatusOK,
		},
		{
			from:   "Plus+Is+Not+Space/" + url.PathEscape("%3Fis?and#afile"),
			to:     "branch/Plus+Is+Not+Space/" + url.PathEscape("%3Fis?and#afile"),
			status: http.StatusOK,
		},
		{
			from:   "Plus+Is+Not+Space/10%25.md",
			to:     "branch/Plus+Is+Not+Space/10%25.md",
			status: http.StatusOK,
		},
		{
			from:   "Plus+Is+Not+Space/" + url.PathEscape("This+file%20has 1space"),
			to:     "branch/Plus+Is+Not+Space/" + url.PathEscape("This+file%20has 1space"),
			status: http.StatusOK,
		},
		{
			from:   "Plus+Is+Not+Space/" + url.PathEscape("This+file%2520has 2 spaces"),
			to:     "branch/Plus+Is+Not+Space/" + url.PathEscape("This+file%2520has 2 spaces"),
			status: http.StatusOK,
		},
		{
			from:   "Plus+Is+Not+Space/" + url.PathEscape("£15&$6.txt"),
			to:     "branch/Plus+Is+Not+Space/" + url.PathEscape("£15&$6.txt"),
			status: http.StatusOK,
		},
	}

	defer tests.PrepareTestEnv(t)()

	user := "user2"
	repo := "utf8"
	session := loginUser(t, user)

	setDefaultBranch(t, session, user, repo, "Plus+Is+Not+Space")
	defer setDefaultBranch(t, session, user, repo, "master")

	for _, test := range testRedirects {
		t.Run(test.from, func(t *testing.T) {
			req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/src/%s", user, repo, test.from))
			resp := session.MakeRequest(t, req, http.StatusSeeOther)
			require.Equal(t, http.StatusSeeOther, resp.Code)

			redirectLocation := resp.Header().Get("Location")
			require.Equal(t, fmt.Sprintf("/%s/%s/src/%s", user, repo, test.to), redirectLocation)

			req = NewRequest(t, "GET", redirectLocation)
			session.MakeRequest(t, req, test.status)
		})
	}
}
