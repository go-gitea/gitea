// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
	"gitea.dev/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullDiff(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("CompletePRDiff", func(t *testing.T) {
		testPullDiffAssertPage(t, "/user2/commitsonpr/pulls/1/files", false, []string{"test1.txt", "test10.txt", "test2.txt", "test3.txt", "test4.txt", "test5.txt", "test6.txt", "test7.txt", "test8.txt", "test9.txt"})
	})
	t.Run("SingleCommitPRDiff", func(t *testing.T) {
		testPullDiffAssertPage(t, "/user2/commitsonpr/pulls/1/commits/c5626fc9eff57eb1bb7b796b01d4d0f2f3f792a2", true, []string{"test3.txt"})
	})
	t.Run("CommitRangePRDiff", func(t *testing.T) {
		testPullDiffAssertPage(t, "/user2/commitsonpr/pulls/1/files/4ca8bcaf27e28504df7bf996819665986b01c847..23576dd018294e476c06e569b6b0f170d0558705", true, []string{"test2.txt", "test3.txt", "test4.txt"})
	})
	t.Run("SingleHeadCommitReviewFormAction", testPullDiffSingleHeadCommitReviewFormAction)
}

func TestLongLinePlainDiff(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	const oldPath, newPath = "old&file.txt", "new&file.txt"
	longLine := strings.Repeat("x", setting.Git.MaxGitDiffLineCharacters+1)
	baseContent, headContent := longLine+"\nold\n", longLine+"\nnew\n"
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
	masterID, err := git.GetBranchCommitID(t.Context(), repo, "master")
	require.NoError(t, err)
	fastImport := fmt.Sprintf(`commit refs/heads/master
mark :1
committer User <user@example.com> 1700000000 +0000
data 0
from %s
M 100644 inline "%s"
data %d
%sM 100644 inline unrelated.txt
data 4
old

commit refs/heads/long-line
committer User <user@example.com> 1700000001 +0000
data 0
from :1
R "%s" "%s"
M 100644 inline "%s"
data %d
%sM 100644 inline unrelated.txt
data 8
changed
`, masterID, oldPath, len(baseContent), baseContent, oldPath, newPath, newPath, len(headContent), headContent)
	_, _, runErr := gitcmd.NewCommand("fast-import").WithRepo(repo).WithStdinBytes([]byte(fastImport)).RunStdString(t.Context())
	require.NoError(t, runErr)

	apiContext := NewAPITestContext(t, repo.OwnerName, repo.Name, auth_model.AccessTokenScopeWriteRepository)
	pullRequest, err := doAPICreatePullRequest(apiContext, repo.OwnerName, repo.Name, repo.DefaultBranch, "long-line")(t)
	require.NoError(t, err)
	headID, err := git.GetBranchCommitID(t.Context(), repo, "long-line")
	require.NoError(t, err)
	session := apiContext.Session
	pullPath := fmt.Sprintf("/%s/%s/pulls/%d/files", repo.OwnerName, repo.Name, pullRequest.Index)
	query := url.Values{"file-only": {"true"}, "plain": {"true"}, "files": {newPath, oldPath}}.Encode()

	t.Run("link", func(t *testing.T) {
		resp := session.MakeRequest(t, NewRequest(t, http.MethodGet, pullPath), http.StatusOK)
		file := NewHTMLParser(t, resp.Body).Find(`.diff-file-box[data-new-filename="new&file.txt"]`)
		require.Equal(t, 1, file.Length())
		suppressed := file.Find(".diff-file-body > .file-body.code-diff > .tw-p-3")
		require.Equal(t, 1, suppressed.Length())
		link := suppressed.Find(`a[href*="plain=true"]`)
		require.Equal(t, 1, link.Length())
		href, _ := link.Attr("href")
		plainURL, err := url.Parse(href)
		require.NoError(t, err)
		assert.Equal(t, []string{newPath, oldPath}, plainURL.Query()["files"])
		assert.Equal(t, "_blank", link.AttrOr("target", ""))
		assert.Equal(t, "nofollow", link.AttrOr("rel", ""))
	})

	for name, path := range map[string]string{
		"commit":  "/user2/repo1/commit/" + headID,
		"pull":    pullPath,
		"compare": "/user2/repo1/compare/master...long-line",
	} {
		t.Run(name, func(t *testing.T) {
			resp := session.MakeRequest(t, NewRequest(t, http.MethodGet, path+"?"+query), http.StatusOK)
			body := resp.Body.String()
			assert.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))
			if !strings.Contains(body, " "+longLine+"\n") {
				t.Error("plain diff omitted complete long line")
			}
			assert.NotContains(t, body, "unrelated.txt")
			assert.NotContains(t, body, "<div")
		})
	}

	for name, files := range map[string][]string{"no files": nil, "empty file": {newPath, ""}, "three files": {newPath, oldPath, "unrelated.txt"}} {
		t.Run(name, func(t *testing.T) {
			values := url.Values{"plain": {"true"}, "file-only": {"true"}, "files": files}
			resp := session.MakeRequest(t, NewRequest(t, http.MethodGet, "/user2/repo1/commit/"+headID+"?"+values.Encode()), NoExpectedStatus)
			assert.Equal(t, http.StatusBadRequest, resp.Code)
		})
	}
}

func testPullDiffSingleHeadCommitReviewFormAction(t *testing.T) {
	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/commitsonpr/pulls/1/commits/1978192d98bb1b65e11c2cf37da854fbf94bffd6")
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)

	btn := doc.Find(".js-btn-review")
	assert.True(t, btn.Length() == 1 && !btn.HasClass("disabled"))
	form := doc.Find(".review-box-panel form")
	assert.Equal(t, 1, form.Length())
	assert.Equal(t, "/user2/commitsonpr/pulls/1/files/reviews/submit", form.AttrOr("action", ""))
}

func testPullDiffAssertPage(t *testing.T, prDiffURL string, reviewBtnDisabled bool, expectedFilenames []string) {
	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/commitsonpr/pulls")
	session.MakeRequest(t, req, http.StatusOK)

	// Get the given PR diff url
	req = NewRequest(t, "GET", prDiffURL)
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)

	// Assert all files are visible.
	fileContents := doc.Find(".file-content")
	numberOfFiles := fileContents.Length()

	assert.Equal(t, len(expectedFilenames), numberOfFiles)

	fileContents.Each(func(i int, s *goquery.Selection) {
		filename, _ := s.Attr("data-old-filename")
		assert.Equal(t, expectedFilenames[i], filename)
	})

	// Ensure the review button is enabled for full PR reviews
	assert.Equal(t, reviewBtnDisabled, doc.Find(".js-btn-review").HasClass("disabled"))
}
