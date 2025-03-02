// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func assertFileExist(t *testing.T, p string) {
	exist, err := util.IsExist(p)
	assert.NoError(t, err)
	assert.True(t, exist)
}

func assertFileEqual(t *testing.T, p string, content []byte) {
	bs, err := os.ReadFile(p)
	assert.NoError(t, err)
	assert.EqualValues(t, content, bs)
}

func TestRepoCloneWiki(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		defer tests.PrepareTestEnv(t)()

		dstPath := t.TempDir()

		r := fmt.Sprintf("%suser2/repo1.wiki.git", u.String())
		u, _ = url.Parse(r)
		u.User = url.UserPassword("user2", userPassword)
		t.Run("Clone", func(t *testing.T) {
			assert.NoError(t, git.CloneWithArgs(t.Context(), git.AllowLFSFiltersArgs(), u.String(), dstPath, git.CloneRepoOptions{}))
			assertFileEqual(t, filepath.Join(dstPath, "Home.md"), []byte("# Home page\n\nThis is the home page!\n"))
			assertFileExist(t, filepath.Join(dstPath, "Page-With-Image.md"))
			assertFileExist(t, filepath.Join(dstPath, "Page-With-Spaced-Name.md"))
			assertFileExist(t, filepath.Join(dstPath, "images"))
			assertFileExist(t, filepath.Join(dstPath, "files/Non-Renderable-File.zip"))
			assertFileExist(t, filepath.Join(dstPath, "jpeg.jpg"))
		})
	})
}

func Test_RepoWikiPages(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	url := "/user2/repo1/wiki/?action=_pages"
	req := NewRequest(t, "GET", url)
	resp := MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	expectedPagePaths := []string{
		"Home", "Page-With-Image", "Page-With-Spaced-Name", "Unescaped-File",
	}
	doc.Find("tr").Each(func(i int, s *goquery.Selection) {
		firstAnchor := s.Find("a").First()
		href, _ := firstAnchor.Attr("href")
		pagePath := strings.TrimPrefix(href, "/user2/repo1/wiki/")

		assert.EqualValues(t, expectedPagePaths[i], pagePath)
	})
}
