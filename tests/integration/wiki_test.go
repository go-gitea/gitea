// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoWikiPages(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequest(t, "GET", "/user2/repo1/wiki/?action=_pages")
	resp := MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	expectedPagePaths := []string{
		"Home", "Page-With-Image", "Page-With-Spaced-Name", "Unescaped-File",
	}
	doc.Find("tr").Each(func(i int, s *goquery.Selection) {
		firstAnchor := s.Find("a").First()
		href, _ := firstAnchor.Attr("href")
		pagePath := strings.TrimPrefix(href, "/user2/repo1/wiki/")

		assert.Equal(t, expectedPagePaths[i], pagePath)
	})
}

func testRepoWikiCloneHTTP(t *testing.T, u *url.URL) {
	// When proc-receive support is enabled globally, the HTTP receive-pack pre-check
	// must still require write access for wiki repositories. Exercise this with a
	// normal wiki push because the regression is about the pre-check, not agit refs.
	require.True(t, git.DefaultFeatures().SupportProcReceive) // modern git should all support proc-receive

	wikiURL := *u
	wikiURL.Path = "/user2/repo1.wiki.git"

	dstLocalPath := t.TempDir()

	// reader can clone
	wikiURL.User = url.UserPassword("user20", userPassword)
	require.NoError(t, git.Clone(t.Context(), wikiURL.String(), dstLocalPath, git.CloneRepoOptions{}))
	_, _, runErr := gitcmd.NewCommand("fast-import").WithDir(dstLocalPath).WithStdinBytes([]byte(`commit refs/heads/master
committer unauthorized-user <user20@example.com> 1714310400 +0000
data <<EOM
dummy-message
EOM
from refs/heads/master^0
M 100644 inline Home.md
data <<EOF
changed-content
EOF
`)).RunStdString(t.Context())
	require.NoError(t, runErr)

	content, err := os.ReadFile(filepath.Join(dstLocalPath, "Home.md"))
	assert.NoError(t, err)
	assert.Equal(t, "# Home page\n\nThis is the home page!\n", string(content))

	// reader can't push
	_, _, runErr = gitcmd.NewCommand("push", "origin", "refs/heads/master").WithDir(dstLocalPath).RunStdString(t.Context())
	assert.True(t, gitcmd.StderrContains(runErr, "remote: Repository not found\n"))
	req := NewRequest(t, "GET", "/user2/repo1/wiki/raw/Home.md")
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "This is the home page!")

	// owner can push
	wikiURL.User = url.UserPassword("user2", userPassword)
	_, _, runErr = gitcmd.NewCommand("remote", "add", "origin-owner").AddDynamicArguments(wikiURL.String()).WithDir(dstLocalPath).RunStdString(t.Context())
	require.NoError(t, runErr)
	_, _, runErr = gitcmd.NewCommand("push", "origin-owner", "refs/heads/master").WithDir(dstLocalPath).RunStdString(t.Context())
	assert.NoError(t, runErr)
	req = NewRequest(t, "GET", "/user2/repo1/wiki/raw/Home.md")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "changed-content", strings.TrimSpace(resp.Body.String()))
}

func testRepoWikiCloneSSH(t *testing.T, u *url.URL) {
	dstLocalPath := t.TempDir()
	baseAPITestContext := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
	sshURL := createSSHUrl("/user2/repo1.wiki.git", u)

	withKeyFile(t, "my-testing-key", func(keyFile string) {
		t.Run("CreateUserKey", doAPICreateUserKey(baseAPITestContext, "test-key", keyFile))
		assert.NoError(t, git.Clone(t.Context(), sshURL.String(), dstLocalPath, git.CloneRepoOptions{}))
		content, err := os.ReadFile(filepath.Join(dstLocalPath, "Home.md"))
		assert.NoError(t, err)
		assert.Equal(t, "# Home page\n\nThis is the home page!\n", string(content))
	})
}

func TestRepoWikiClonePush(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		t.Run("SSH", func(t *testing.T) { testRepoWikiCloneSSH(t, u) })
		t.Run("HTTP", func(t *testing.T) { testRepoWikiCloneHTTP(t, u) })
	})
}
