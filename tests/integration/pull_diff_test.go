// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/services/gitdiff"
	"gitea.dev/tests"

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
	t.Run("RangeNotInPR", func(t *testing.T) {
		session := loginUser(t, "user2")
		session.MakeRequest(t, NewRequest(t, "GET", "/user2/commitsonpr/pulls/1/files/stream?to=65f1bf27bc3bf70f64657658635e66094edbcb4d"), http.StatusNotFound)
	})
	t.Run("SingleHeadCommitReviewFormAction", testPullDiffSingleHeadCommitReviewFormAction)
	t.Run("Comments", func(t *testing.T) {
		session := loginUser(t, "user2")
		commentIDs := func(showOutdated string) map[int64]bool {
			resp := session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/pulls/2/files/comments?show-outdated="+showOutdated), http.StatusOK)
			data := DecodeJSON(t, resp, &struct {
				CanComment bool `json:"canComment"`
				Comments   []struct {
					ID       int64  `json:"id"`
					Side     string `json:"side"`
					Line     int    `json:"line"`
					Outdated bool   `json:"outdated"`
				} `json:"comments"`
			}{})
			assert.True(t, data.CanComment)
			ids := map[int64]bool{}
			for _, comment := range data.Comments {
				assert.Equal(t, "old", comment.Side)
				assert.Equal(t, 4, comment.Line)
				ids[comment.ID] = comment.Outdated
			}
			return ids
		}
		// comment 4 belongs to user1's pending review
		assert.Equal(t, map[int64]bool{5: false, 6: true}, commentIDs("true"))
		assert.Equal(t, map[int64]bool{5: false}, commentIDs("false"))

		// the diff viewer gets an error instead of a redirect, so it can keep the draft
		req := NewRequestWithValues(t, "POST", "/user2/repo1/pulls/2/files/reviews/comments", map[string]string{"origin": "diff", "side": "proposed", "line": "1", "path": "README.md"})
		resp := session.MakeRequest(t, req, http.StatusBadRequest)
		assert.NotEmpty(t, DecodeJSON(t, resp, &struct {
			ErrorMessage string `json:"errorMessage"`
		}{}).ErrorMessage)
	})
	t.Run("Anonymous", func(t *testing.T) {
		MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/pulls/2/files"), http.StatusOK)
		resp := MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/pulls/2/files/comments"), http.StatusOK)
		assert.Contains(t, resp.Body.String(), `"canComment":false`)
	})
}

type testDiffViewerConfig struct {
	StreamURL    string `json:"streamUrl"`
	EditFileLink string `json:"editFileLink"`
	RepoLink     string `json:"repoLink"`
}

// diffViewerConfigOf returns the diff viewer's page data of a diff page
func diffViewerConfigOf(t *testing.T, htmlDoc *HTMLDoc) *testDiffViewerConfig {
	var windowConfig struct {
		PageData struct {
			DiffViewer *testDiffViewerConfig `json:"diffViewer"`
		} `json:"pageData"`
	}
	require.NoError(t, json.Unmarshal([]byte(htmlDoc.Find("#global-window-config").Text()), &windowConfig))
	require.NotNil(t, windowConfig.PageData.DiffViewer)
	return windowConfig.PageData.DiffViewer
}

// requestDiffStream returns the files of a diff stream and whether it was truncated
func requestDiffStream(t *testing.T, session *TestSession, streamURL string) (files []*gitdiff.DiffStreamFile, truncated bool) {
	resp := session.MakeRequest(t, NewRequest(t, "GET", streamURL), http.StatusOK)
	assert.Equal(t, "application/x-ndjson", resp.Header().Get("Content-Type"))
	lines := strings.Split(strings.TrimSpace(resp.Body.String()), "\n")
	for _, line := range lines[:len(lines)-1] {
		file := &gitdiff.DiffStreamFile{}
		require.NoError(t, json.Unmarshal([]byte(line), file))
		files = append(files, file)
	}
	var end struct {
		End       bool `json:"end"`
		Truncated bool `json:"truncated"`
		Failed    bool `json:"failed"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &end))
	require.True(t, end.End)
	require.False(t, end.Failed)
	return files, end.Truncated
}

func testPullDiffSingleHeadCommitReviewFormAction(t *testing.T) {
	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/commitsonpr/pulls/1/commits/1978192d98bb1b65e11c2cf37da854fbf94bffd6")
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)

	btn := doc.Find(".review-box-button")
	require.Equal(t, 1, btn.Length())
	assert.False(t, btn.Is("[disabled]"))
	form := doc.Find(".review-box-panel form")
	assert.Equal(t, 1, form.Length())
	assert.Equal(t, "/user2/commitsonpr/pulls/1/files/reviews/submit", form.AttrOr("action", ""))
}

func testPullDiffAssertPage(t *testing.T, prDiffURL string, reviewBtnDisabled bool, expectedFilenames []string) {
	session := loginUser(t, "user2")
	resp := session.MakeRequest(t, NewRequest(t, "GET", prDiffURL), http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)

	// Ensure the review button is enabled for full PR reviews
	btn := doc.Find(".review-box-button")
	require.Equal(t, 1, btn.Length())
	assert.Equal(t, reviewBtnDisabled, btn.Is("[disabled]"))

	files, truncated := requestDiffStream(t, session, diffViewerConfigOf(t, doc).StreamURL)
	assert.False(t, truncated)
	var names []string
	for _, file := range files {
		names = append(names, file.OldName)
		assert.NotEmpty(t, file.NameHash)
	}
	assert.Equal(t, expectedFilenames, names)
}

func TestDiffStream(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	files, _ := requestDiffStream(t, session, "/user2/repo1/diff/stream?after=65f1bf27bc3bf70f64657658635e66094edbcb4d")
	require.Len(t, files, 1)
	assert.Equal(t, "README.md", files[0].Name)
	assert.Equal(t, "added", files[0].Type)
	assert.Contains(t, files[0].Hunks, "+# repo1")

	files, _ = requestDiffStream(t, session, "/user2/repo1/wiki/diff/stream?after=a5bbc0fd39a696feabed2d4cccaf05abbcaf3b02")
	require.Len(t, files, 1)
	assert.Equal(t, "files/Non-Renderable-File.zip", files[0].Name)
	assert.True(t, files[0].IsBinary)

	session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/diff/stream?after=0000000000000000000000000000000000000000"), http.StatusNotFound)

	t.Run("Limits", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Git.MaxDiffViewFileSize, 10)()
		files, _ := requestDiffStream(t, session, "/user2/repo1/diff/stream?after=65f1bf27bc3bf70f64657658635e66094edbcb4d")
		assert.True(t, files[0].TooLarge)
		assert.Empty(t, files[0].Hunks)
		files, _ = requestDiffStream(t, session, "/user2/repo1/diff/stream?after=65f1bf27bc3bf70f64657658635e66094edbcb4d&full=true")
		assert.True(t, files[0].TooLarge) // only a requested file is loaded in full
		files, _ = requestDiffStream(t, session, "/user2/repo1/diff/stream?after=65f1bf27bc3bf70f64657658635e66094edbcb4d&full=true&files=README.md")
		assert.False(t, files[0].TooLarge)

		// a file beyond the total limit can't be loaded in full either
		defer test.MockVariableValue(&setting.Git.MaxDiffViewSize, 10)()
		files, truncated := requestDiffStream(t, session, "/user2/repo1/diff/stream?after=65f1bf27bc3bf70f64657658635e66094edbcb4d&full=true&files=README.md")
		require.Len(t, files, 1)
		assert.True(t, files[0].TooLarge)
		assert.False(t, truncated)
	})

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	fastImport := func(t *testing.T, before, after []git.FastImportFile) string {
		ref := "refs/heads/" + t.Name()
		require.NoError(t, git.ForceFastImport(t.Context(), repo.CodeStorageRepo(), []git.FastImportCommit{{Ref: ref + "-before", Files: before}, {Ref: ref + "-after", Files: after}}))
		return "/user2/repo1/diff/stream?" + url.Values{"before": {ref + "-before"}, "after": {ref + "-after"}}.Encode()
	}

	t.Run("LongLine", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Git.MaxGitDiffLineCharacters, 32)()
		longLine := strings.Repeat("long ", 32)
		streamURL := fastImport(t, []git.FastImportFile{{Path: "long.txt", Content: "old\n"}}, []git.FastImportFile{{Path: "long.txt", Content: longLine + "\n"}})
		// the diff viewer doesn't truncate lines, the limit only applies to API diffs and snippets
		files, _ := requestDiffStream(t, session, streamURL)
		require.Len(t, files, 1)
		assert.Contains(t, files[0].Hunks, "+"+longLine+"\n")
	})

	t.Run("Submodule", func(t *testing.T) {
		oldCommit, newCommit := strings.Repeat("1", 40), strings.Repeat("2", 40)
		gitmodules := git.FastImportFile{Path: ".gitmodules", Content: "[submodule \"sub\"]\n\tpath = sub\n\turl = https://gitea.example.com/org/sub.git\n"}
		streamURL := fastImport(t,
			[]git.FastImportFile{gitmodules, {Mode: git.EntryModeCommit, Path: "sub", Content: oldCommit}},
			[]git.FastImportFile{gitmodules, {Mode: git.EntryModeCommit, Path: "sub", Content: newCommit}},
		)
		files, _ := requestDiffStream(t, session, streamURL)
		require.Len(t, files, 1)
		require.NotNil(t, files[0].Submodule)
		assert.Equal(t, oldCommit, files[0].Submodule.OldCommit)
		assert.Equal(t, newCommit, files[0].Submodule.NewCommit)
		assert.Equal(t, "https://gitea.example.com/org/sub", files[0].Submodule.RepoLink)
		assert.Equal(t, "https://gitea.example.com/org/sub/compare/"+oldCommit+"..."+newCommit, files[0].Submodule.CompareLink)
	})
}
