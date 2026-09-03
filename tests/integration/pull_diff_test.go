// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git/gitcmd"
	api "gitea.dev/modules/structs"
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

func TestPullDiffNoCommonMergeBase(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user2.ID, Name: "repo1"})
	_, _, err := gitcmd.NewCommand("fast-import").WithRepo(repo1).WithStdinBytes([]byte(strings.TrimSpace(`
commit refs/heads/unrelated-history
committer User <user@example.com> 1714310400 +0000
data 13
Second commit
M 100644 inline file2.txt
data 12
Hello from 2
`))).RunStdString(t.Context())
	require.NoError(t, err)

	// the compare page refuses branches without a merge base, but the API (and history rewrites) still produce such pull requests
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/pulls", &api.CreatePullRequestOption{
		Head:  "unrelated-history",
		Base:  "master",
		Title: "unrelated histories",
	}).AddTokenAuth(token)
	pr := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.PullRequest{})

	req = NewRequest(t, "GET", fmt.Sprintf("/user2/repo1/pulls/%d/files", pr.Index))
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)
	assert.Equal(t, 1, doc.Find(".diff-file-box").Length())
	assert.Equal(t, "file2.txt", doc.Find(".diff-file-box").AttrOr("data-new-filename", ""))
}
