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
	t.Run("CommitRangePRDiff", func(t *testing.T) {
		testPullDiffAssertPage(t, "/user2/commitsonpr/pulls/1/files/4ca8bcaf27e28504df7bf996819665986b01c847..23576dd018294e476c06e569b6b0f170d0558705", true, []string{"test2.txt", "test3.txt", "test4.txt"})
	})
	t.Run("SingleCommit", testPullDiffSingleCommit)
}

func testPullDiffSingleCommit(t *testing.T) {
	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/commitsonpr/pulls/1/commits/c5626fc9eff57eb1bb7b796b01d4d0f2f3f792a2")
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)

	bar := doc.Find("[data-commit-single-index]")
	require.Equal(t, 1, bar.Length())
	barText := bar.Text()
	assert.Contains(t, barText, "test 3")
	assert.Contains(t, barText, "Sebastian Sauer")
	assert.Equal(t, "3", bar.AttrOr("data-commit-single-index", ""))
	assert.Equal(t, "10", bar.AttrOr("data-commit-total-count", ""))

	assert.Contains(t, bar.Find(".js-previous-commit-link").AttrOr("href", ""), "/pulls/1/commits/96cef4a7b72b3c208340ae6f0cf55a93e9077c93")
	assert.Contains(t, bar.Find(".js-next-commit-link").AttrOr("href", ""), "/pulls/1/commits/23576dd018294e476c06e569b6b0f170d0558705")

	commitSelect := doc.Find("#diff-commit-select")
	assert.Equal(t, "96cef4a7b72b3c208340ae6f0cf55a93e9077c93", commitSelect.AttrOr("data-before-commit", ""))
	assert.Equal(t, "c5626fc9eff57eb1bb7b796b01d4d0f2f3f792a2", commitSelect.AttrOr("data-after-commit", ""))

	panel := doc.Find(".review-box-panel form")
	require.Equal(t, 1, panel.Length())
	assert.Equal(t, "c5626fc9eff57eb1bb7b796b01d4d0f2f3f792a2", panel.Find(`input[name="commit_id"]`).AttrOr("value", ""))

	req = NewRequest(t, "GET", "/user2/commitsonpr/pulls/1/files/reviews/new_comment")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "1978192d98bb1b65e11c2cf37da854fbf94bffd6", NewHTMLParser(t, resp.Body).Find(`input[name="latest_commit_id"]`).AttrOr("value", ""))

	req = NewRequest(t, "GET", "/user2/commitsonpr/pulls/1/commits/4ca8bcaf27e28504df7bf996819665986b01c847")
	resp = session.MakeRequest(t, req, http.StatusOK)
	bar = NewHTMLParser(t, resp.Body).Find("[data-commit-single-index]")
	assert.Equal(t, "1", bar.AttrOr("data-commit-single-index", ""))
	assert.True(t, bar.Find(".js-previous-commit-link").HasClass("disabled"))
	assert.False(t, bar.Find(".js-next-commit-link").HasClass("disabled"))

	req = NewRequest(t, "GET", "/user2/commitsonpr/pulls/1/commits/cbff181af4c9c7fee3cf6c106699e07d9a3f54e6")
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/user2/commitsonpr/commit/cbff181af4c9c7fee3cf6c106699e07d9a3f54e6", test.RedirectURL(resp))
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
	btn := doc.Find(".review-box-button")
	require.Equal(t, 1, btn.Length())
	assert.Equal(t, reviewBtnDisabled, btn.Is("[disabled]"))
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
