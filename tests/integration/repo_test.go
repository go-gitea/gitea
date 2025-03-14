// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"path"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func TestViewRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo1")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	repoTopics := htmlDoc.doc.Find("#repo-topics").Children()
	repoSummary := htmlDoc.doc.Find(".repository-summary").Children()

	assert.True(t, repoTopics.HasClass("repo-topic"))
	assert.True(t, repoSummary.HasClass("repository-menu"))

	req = NewRequest(t, "GET", "/org3/repo3")
	MakeRequest(t, req, http.StatusNotFound)

	session = loginUser(t, "user1")
	session.MakeRequest(t, req, http.StatusNotFound)
}

func testViewRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/org3/repo3")
	session := loginUser(t, "user2")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	files := htmlDoc.doc.Find("#repo-files-table .repo-file-item")

	type file struct {
		fileName   string
		commitID   string
		commitMsg  string
		commitTime string
	}

	var items []file

	files.Each(func(i int, s *goquery.Selection) {
		tds := s.Find(".repo-file-cell")
		var f file
		tds.Each(func(i int, s *goquery.Selection) {
			if i == 0 {
				f.fileName = strings.TrimSpace(s.Text())
			} else if i == 1 {
				a := s.Find("a")
				f.commitMsg = strings.TrimSpace(a.Text())
				l, _ := a.Attr("href")
				f.commitID = path.Base(l)
			}
		})

		// convert "2017-06-14 21:54:21 +0800" to "Wed, 14 Jun 2017 13:54:21 UTC"
		htmlTimeString, _ := s.Find("relative-time").Attr("datetime")
		htmlTime, _ := time.Parse(time.RFC3339, htmlTimeString)
		f.commitTime = htmlTime.In(time.Local).Format(time.RFC1123)
		items = append(items, f)
	})

	commitT := time.Date(2017, time.June, 14, 13, 54, 21, 0, time.UTC).In(time.Local).Format(time.RFC1123)
	assert.EqualValues(t, []file{
		{
			fileName:   "doc",
			commitID:   "2a47ca4b614a9f5a43abbd5ad851a54a616ffee6",
			commitMsg:  "init project",
			commitTime: commitT,
		},
		{
			fileName:   "README.md",
			commitID:   "2a47ca4b614a9f5a43abbd5ad851a54a616ffee6",
			commitMsg:  "init project",
			commitTime: commitT,
		},
	}, items)
}

func TestViewRepo2(t *testing.T) {
	// no last commit cache
	testViewRepo(t)

	// enable last commit cache for all repositories
	oldCommitsCount := setting.CacheService.LastCommit.CommitsCount
	setting.CacheService.LastCommit.CommitsCount = 0
	// first view will not hit the cache
	testViewRepo(t)
	// second view will hit the cache
	testViewRepo(t)
	setting.CacheService.LastCommit.CommitsCount = oldCommitsCount
}

func TestViewRepo3(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/org3/repo3")
	session := loginUser(t, "user4")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestViewRepo1CloneLinkAnonymous(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo1")
	resp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find(".repo-clone-https").Attr("data-link")
	assert.True(t, exists, "The template has changed")
	assert.Equal(t, setting.AppURL+"user2/repo1.git", link)

	_, exists = htmlDoc.doc.Find(".repo-clone-ssh").Attr("data-link")
	assert.False(t, exists)

	link, exists = htmlDoc.doc.Find(".repo-clone-tea").Attr("data-link")
	assert.True(t, exists, "The template has changed")
	assert.Equal(t, "tea clone user2/repo1", link)
}

func TestViewRepo1CloneLinkAuthorized(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo1")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find(".repo-clone-https").Attr("data-link")
	assert.True(t, exists, "The template has changed")
	assert.Equal(t, setting.AppURL+"user2/repo1.git", link)

	link, exists = htmlDoc.doc.Find(".repo-clone-ssh").Attr("data-link")
	assert.True(t, exists, "The template has changed")
	sshURL := fmt.Sprintf("ssh://%s@%s:%d/user2/repo1.git", setting.SSH.User, setting.SSH.Domain, setting.SSH.Port)
	assert.Equal(t, sshURL, link)

	link, exists = htmlDoc.doc.Find(".repo-clone-tea").Attr("data-link")
	assert.True(t, exists, "The template has changed")
	assert.Equal(t, "tea clone user2/repo1", link)
}

func TestViewRepoWithSymlinks(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.UI.FileIconTheme, "basic")()
	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo20.git")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	files := htmlDoc.doc.Find("#repo-files-table .repo-file-cell.name")
	items := files.Map(func(i int, s *goquery.Selection) string {
		cls, _ := s.Find("SVG").Attr("class")
		file := strings.Trim(s.Find("A").Text(), " \t\n")
		return fmt.Sprintf("%s: %s", file, cls)
	})
	assert.Len(t, items, 5)
	assert.Equal(t, "a: svg octicon-file-directory-fill", items[0])
	assert.Equal(t, "link_b: svg octicon-file-directory-symlink", items[1])
	assert.Equal(t, "link_d: svg octicon-file-symlink-file", items[2])
	assert.Equal(t, "link_hi: svg octicon-file-symlink-file", items[3])
	assert.Equal(t, "link_link: svg octicon-file-symlink-file", items[4])
}

// TestViewFileInRepo repo description, topics and summary should not be displayed when viewing a file
func TestViewFileInRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo1/src/branch/master/README.md")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	description := htmlDoc.doc.Find(".repo-description")
	repoTopics := htmlDoc.doc.Find("#repo-topics")
	repoSummary := htmlDoc.doc.Find(".repository-summary")

	assert.EqualValues(t, 0, description.Length())
	assert.EqualValues(t, 0, repoTopics.Length())
	assert.EqualValues(t, 0, repoSummary.Length())
}

// TestBlameFileInRepo repo description, topics and summary should not be displayed when running blame on a file
func TestBlameFileInRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo1/blame/branch/master/README.md")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	description := htmlDoc.doc.Find(".repo-description")
	repoTopics := htmlDoc.doc.Find("#repo-topics")
	repoSummary := htmlDoc.doc.Find(".repository-summary")

	assert.EqualValues(t, 0, description.Length())
	assert.EqualValues(t, 0, repoTopics.Length())
	assert.EqualValues(t, 0, repoSummary.Length())
}

// TestViewRepoDirectory repo description, topics and summary should not be displayed when within a directory
func TestViewRepoDirectory(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo20/src/branch/master/a")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	description := htmlDoc.doc.Find(".repo-description")
	repoTopics := htmlDoc.doc.Find("#repo-topics")
	repoSummary := htmlDoc.doc.Find(".repository-summary")

	repoFilesTable := htmlDoc.doc.Find("#repo-files-table")
	assert.NotEmpty(t, repoFilesTable.Nodes)

	assert.Zero(t, description.Length())
	assert.Zero(t, repoTopics.Length())
	assert.Zero(t, repoSummary.Length())
}

// ensure that the all the different ways to find and render a README work
func TestViewRepoDirectoryReadme(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// there are many combinations:
	// - READMEs can be .md, .txt, or have no extension
	// - READMEs can be tagged with a language and even a country code
	// - READMEs can be stored in docs/, .gitea/, or .github/
	// - READMEs can be symlinks to other files
	// - READMEs can be broken symlinks which should not render
	//
	// this doesn't cover all possible cases, just the major branches of the code

	session := loginUser(t, "user2")

	check := func(name, url, expectedFilename, expectedReadmeType, expectedContent string) {
		t.Run(name, func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", url)
			resp := session.MakeRequest(t, req, http.StatusOK)

			htmlDoc := NewHTMLParser(t, resp.Body)
			readmeName := htmlDoc.doc.Find("h4.file-header")
			readmeContent := htmlDoc.doc.Find(".file-view") // TODO: add a id="readme" to the output to make this test more precise
			readmeType, _ := readmeContent.Attr("class")

			assert.Equal(t, expectedFilename, strings.TrimSpace(readmeName.Text()))
			assert.Contains(t, readmeType, expectedReadmeType)
			assert.Contains(t, readmeContent.Text(), expectedContent)
		})
	}

	// viewing the top level
	check("Home", "/user2/readme-test/", "README.md", "markdown", "The cake is a lie.")

	// viewing different file extensions
	check("md", "/user2/readme-test/src/branch/master/", "README.md", "markdown", "The cake is a lie.")
	check("txt", "/user2/readme-test/src/branch/txt/", "README.txt", "plain-text", "My spoon is too big.")
	check("plain", "/user2/readme-test/src/branch/plain/", "README", "plain-text", "Birken my stocks gee howdy")
	check("i18n", "/user2/readme-test/src/branch/i18n/", "README.zh.md", "markdown", "蛋糕是一个谎言")

	// using HEAD ref
	check("branch-HEAD", "/user2/readme-test/src/branch/HEAD/", "README.md", "markdown", "The cake is a lie.")
	check("commit-HEAD", "/user2/readme-test/src/commit/HEAD/", "README.md", "markdown", "The cake is a lie.")

	// viewing different subdirectories
	check("subdir", "/user2/readme-test/src/branch/subdir/libcake", "README.md", "markdown", "Four pints of sugar.")
	check("docs-direct", "/user2/readme-test/src/branch/special-subdir-docs/docs/", "README.md", "markdown", "This is in docs/")
	check("docs", "/user2/readme-test/src/branch/special-subdir-docs/", "docs/README.md", "markdown", "This is in docs/")
	check(".gitea", "/user2/readme-test/src/branch/special-subdir-.gitea/", ".gitea/README.md", "markdown", "This is in .gitea/")
	check(".github", "/user2/readme-test/src/branch/special-subdir-.github/", ".github/README.md", "markdown", "This is in .github/")

	// symlinks
	// symlinks are subtle:
	// - they should be able to handle going a reasonable number of times up and down in the tree
	// - they shouldn't get stuck on link cycles
	// - they should determine the filetype based on the name of the link, not the target
	check("symlink", "/user2/readme-test/src/branch/symlink/", "README.md", "markdown", "This is in some/other/path")
	check("symlink-multiple", "/user2/readme-test/src/branch/symlink/some/", "README.txt", "plain-text", "This is in some/other/path")
	check("symlink-up-and-down", "/user2/readme-test/src/branch/symlink/up/back/down/down", "README.md", "markdown", "It's a me, mario")

	// testing fallback rules
	// READMEs are searched in this order:
	// - [README.zh-cn.md, README.zh_cn.md, README.zh.md, README_zh.md, README.md, README.txt, README,
	//     docs/README.zh-cn.md, docs/README.zh_cn.md, docs/README.zh.md, docs/README_zh.md, docs/README.md, docs/README.txt, docs/README,
	//    .gitea/README.zh-cn.md, .gitea/README.zh_cn.md, .gitea/README.zh.md, .gitea/README_zh.md, .gitea/README.md, .gitea/README.txt, .gitea/README,

	//     .github/README.zh-cn.md, .github/README.zh_cn.md, .github/README.zh.md, .github/README_zh.md, .github/README.md, .github/README.txt, .github/README]
	// and a broken/looped symlink counts as not existing at all and should be skipped.
	// again, this doesn't cover all cases, but it covers a few
	check("fallback/top", "/user2/readme-test/src/branch/fallbacks/", "README.en.md", "markdown", "This is README.en.md")
	check("fallback/2", "/user2/readme-test/src/branch/fallbacks2/", "README.md", "markdown", "This is README.md")
	check("fallback/3", "/user2/readme-test/src/branch/fallbacks3/", "README", "plain-text", "This is README")
	check("fallback/4", "/user2/readme-test/src/branch/fallbacks4/", "docs/README.en.md", "markdown", "This is docs/README.en.md")
	check("fallback/5", "/user2/readme-test/src/branch/fallbacks5/", "docs/README.md", "markdown", "This is docs/README.md")
	check("fallback/6", "/user2/readme-test/src/branch/fallbacks6/", "docs/README", "plain-text", "This is docs/README")
	check("fallback/7", "/user2/readme-test/src/branch/fallbacks7/", ".gitea/README.en.md", "markdown", "This is .gitea/README.en.md")
	check("fallback/8", "/user2/readme-test/src/branch/fallbacks8/", ".gitea/README.md", "markdown", "This is .gitea/README.md")
	check("fallback/9", "/user2/readme-test/src/branch/fallbacks9/", ".gitea/README", "plain-text", "This is .gitea/README")

	// this case tests that broken symlinks count as missing files, instead of rendering their contents
	check("fallbacks-broken-symlinks", "/user2/readme-test/src/branch/fallbacks-broken-symlinks/", "docs/README", "plain-text", "This is docs/README")

	// some cases that should NOT render a README
	// - /readme
	// - /.github/docs/README.md
	// - a symlink loop

	missing := func(name, url string) {
		t.Run("missing/"+name, func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", url)
			resp := session.MakeRequest(t, req, http.StatusOK)

			htmlDoc := NewHTMLParser(t, resp.Body)
			_, exists := htmlDoc.doc.Find(".file-view").Attr("class")

			assert.False(t, exists, "README should not have rendered")
		})
	}
	missing("sp-ace", "/user2/readme-test/src/branch/sp-ace/")
	missing("nested-special", "/user2/readme-test/src/branch/special-subdir-nested/subproject") // the special subdirs should only trigger on the repo root
	missing("special-subdir-nested", "/user2/readme-test/src/branch/special-subdir-nested/")
	missing("symlink-loop", "/user2/readme-test/src/branch/symlink-loop/")
}

func TestMarkDownReadmeImage(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo1/src/branch/home-md-img-check")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	src, exists := htmlDoc.doc.Find(`.markdown img`).Attr("src")
	assert.True(t, exists, "Image not found in README")
	assert.Equal(t, "/user2/repo1/media/branch/home-md-img-check/test-fake-img.jpg", src)

	req = NewRequest(t, "GET", "/user2/repo1/src/branch/home-md-img-check/README.md")
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	src, exists = htmlDoc.doc.Find(`.markdown img`).Attr("src")
	assert.True(t, exists, "Image not found in markdown file")
	assert.Equal(t, "/user2/repo1/media/branch/home-md-img-check/test-fake-img.jpg", src)
}

func TestMarkDownReadmeImageSubfolder(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// this branch has the README in the special docs/README.md location
	req := NewRequest(t, "GET", "/user2/repo1/src/branch/sub-home-md-img-check")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	src, exists := htmlDoc.doc.Find(`.markdown img`).Attr("src")
	assert.True(t, exists, "Image not found in README")
	assert.Equal(t, "/user2/repo1/media/branch/sub-home-md-img-check/docs/test-fake-img.jpg", src)

	req = NewRequest(t, "GET", "/user2/repo1/src/branch/sub-home-md-img-check/docs/README.md")
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	src, exists = htmlDoc.doc.Find(`.markdown img`).Attr("src")
	assert.True(t, exists, "Image not found in markdown file")
	assert.Equal(t, "/user2/repo1/media/branch/sub-home-md-img-check/docs/test-fake-img.jpg", src)
}

func TestGeneratedSourceLink(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("Rendered file", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", "/user2/repo1/src/branch/master/README.md?display=source")
		resp := MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)

		dataURL, exists := doc.doc.Find(".copy-line-permalink").Attr("data-url")
		assert.True(t, exists)
		assert.Equal(t, "/user2/repo1/src/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d/README.md?display=source", dataURL)

		dataURL, exists = doc.doc.Find(".ref-in-new-issue").Attr("data-url-param-body-link")
		assert.True(t, exists)
		assert.Equal(t, "/user2/repo1/src/commit/65f1bf27bc3bf70f64657658635e66094edbcb4d/README.md?display=source", dataURL)
	})

	t.Run("Non-Rendered file", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		session := loginUser(t, "user27")
		req := NewRequest(t, "GET", "/user27/repo49/src/branch/master/test/test.txt")
		resp := session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)

		dataURL, exists := doc.doc.Find(".copy-line-permalink").Attr("data-url")
		assert.True(t, exists)
		assert.Equal(t, "/user27/repo49/src/commit/aacbdfe9e1c4b47f60abe81849045fa4e96f1d75/test/test.txt", dataURL)

		dataURL, exists = doc.doc.Find(".ref-in-new-issue").Attr("data-url-param-body-link")
		assert.True(t, exists)
		assert.Equal(t, "/user27/repo49/src/commit/aacbdfe9e1c4b47f60abe81849045fa4e96f1d75/test/test.txt", dataURL)
	})
}

func TestViewCommit(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo1/commit/0123456789012345678901234567890123456789")
	req.Header.Add("Accept", "text/html")
	resp := MakeRequest(t, req, http.StatusNotFound)
	assert.True(t, test.IsNormalPageCompleted(resp.Body.String()), "non-existing commit should render 404 page")
}
