// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strings"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
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

func TestLongLineDiffRendering(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Git.MaxGitDiffLineCharacters, 32)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	const suffix = "DISCARDED_SUFFIX"
	const short = "following short line"
	contextLine := strings.Repeat("context ", 8) + suffix + "\n"
	before := git.FastImportCommit{Ref: "refs/heads/long-line-before"}
	after := git.FastImportCommit{Ref: "refs/heads/long-line-after"}
	for _, name := range []string{"long.txt", "long.csv"} {
		before.Files = append(before.Files, git.FastImportFile{Path: name, Content: strings.Repeat("old ", 16) + suffix + "\n" + contextLine + short + "\n"})
		after.Files = append(after.Files, git.FastImportFile{Path: name, Content: strings.Repeat("new ", 16) + suffix + "\n" + contextLine + short + "\n"})
	}
	outsideHunk := "header\n" + contextLine + strings.Repeat("unchanged\n", 6)
	before.Files = append(before.Files, git.FastImportFile{Path: "outside.csv", Content: outsideHunk + "old\n"})
	after.Files = append(after.Files, git.FastImportFile{Path: "outside.csv", Content: outsideHunk + "new\n"})
	require.NoError(t, git.ForceFastImport(t.Context(), repo.CodeStorageRepo(), []git.FastImportCommit{before, after}))

	oldPrefix := strings.Repeat("old ", 8)[:31]
	newPrefix := strings.Repeat("new ", 8)[:31]
	contextPrefix := contextLine[:31]
	for style, want := range map[string][]string{
		"unified": {oldPrefix, newPrefix, contextPrefix, short},
		"split":   {oldPrefix, newPrefix, contextPrefix, contextPrefix, short, short},
	} {
		t.Run(style, func(t *testing.T) {
			req := NewRequest(t, "GET", "/user2/repo1/compare/long-line-before..long-line-after?style="+style)
			resp := MakeRequest(t, req, http.StatusOK)
			doc := NewHTMLParser(t, resp.Body)
			for _, filename := range []string{"long.txt", "long.csv"} {
				t.Run(filename, func(t *testing.T) {
					diffFileBox := doc.Find(`.diff-file-box[data-new-filename="` + filename + `"]`)
					diffBody := diffFileBox.Find(".code-diff-" + style)
					require.Equal(t, 1, diffBody.Length())
					assert.False(t, diffBody.HasClass("tw-hidden"))
					assert.Empty(t, diffFileBox.Find(".file-view-toggle, .data-table").Nodes)
					assert.NotContains(t, diffFileBox.Text(), suffix)
					got := diffBody.Find("tr:not(.tag-code) code.code-inner").Map(func(_ int, s *goquery.Selection) string {
						markerText := s.Find(".diff-line-truncated").Text()
						textContent := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s.Text()), markerText))
						if textContent == short {
							assert.Empty(t, markerText)
						} else {
							assert.Equal(t, "Line truncated", markerText)
						}
						return textContent
					})
					assert.Equal(t, want, got)
				})
			}
			outside := doc.Find(`.diff-file-box[data-new-filename="outside.csv"]`)
			assert.Equal(t, 2, outside.Find(".file-view-toggle").Length())
			assert.Equal(t, 1, outside.Find(".data-table").Length())
		})
	}
}
