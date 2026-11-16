// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/git"
	code_indexer "gitea.dev/modules/indexer/code"
	"gitea.dev/modules/indexer/code/gitgrep"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	commitstatus_service "gitea.dev/services/repository/commitstatus"
	"gitea.dev/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resultFilenames(doc *HTMLDoc) []string {
	filenameSelections := doc.doc.Find(".repository.search").Find(".repo-search-result").Find(".header").Find("span.file")
	result := make([]string, filenameSelections.Length())
	filenameSelections.Each(func(i int, selection *goquery.Selection) {
		result[i] = selection.Text()
	})
	return result
}

func TestSearchRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo1")
	assert.NoError(t, err)

	code_indexer.UpdateRepoIndexer(repo)

	t.Run("ScopedPages", func(t *testing.T) {
		for _, search := range []struct {
			url   string
			count int
		}{
			{"/explore/code?q=Description+repo:user2/repo1", 1},
			{"/explore/code?q=Description+repo:missing/missing", 0},
			{"/user2/-/code?q=Description", 1},
			{"/org3/-/code?q=Description", 0},
		} {
			resp := MakeRequest(t, NewRequest(t, http.MethodGet, search.url), http.StatusOK)
			assert.Equal(t, search.count, NewHTMLParser(t, resp.Body).doc.Find(".repo-search-result").Length(), search.url)
		}
	})

	doc := testSearch(t, "/user2/repo1/search?q=Description&page=1&search_mode=regexp&l=Go", []string{"README.md"})
	assert.Zero(t, doc.doc.Find(`form input[name="search_mode"]`).Length())
	languageLink, exists := doc.doc.Find("a.label").First().Attr("href")
	require.True(t, exists)
	languageURL, err := url.Parse(languageLink)
	require.NoError(t, err)
	assert.Equal(t, url.Values{"q": {"Description language:Markdown"}}, languageURL.Query())

	doc = testSearch(t, "/user2/repo1/search"+languageLink, []string{"README.md"})
	languageLink, exists = doc.doc.Find("a.primary.label").Attr("href")
	require.True(t, exists)
	languageURL, err = url.Parse(languageLink)
	require.NoError(t, err)
	assert.Equal(t, url.Values{"q": {"Description"}}, languageURL.Query())

	doc = testSearch(t, "/user2/repo1/search?q="+url.QueryEscape(`"unterminated`), []string{})
	assert.Contains(t, doc.doc.Find(".ui.error.message").Text(), "Invalid search query:")
	assert.NotContains(t, doc.doc.Find(".ui.list").Text(), "No matching results found.")

	t.Run("GitGrep", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Indexer.RepoIndexerEnabled, false)()
		testSearch(t, "/user2/repo1/search?q="+url.QueryEscape("/Descri.tion/"), []string{"README.md"})
	})

	defer test.MockVariableValue(&setting.Indexer.IncludePatterns, setting.IndexerGlobFromString("**.txt"))()
	defer test.MockVariableValue(&setting.Indexer.ExcludePatterns, setting.IndexerGlobFromString("**/y/**"))()

	repo, err = repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "glob")
	require.NoError(t, err)
	files := []git.FastImportFile{
		{Path: "a.txt", Content: "file1 loren ipsum\n\n\n\nlastMatch\n"},
		{Path: "aaa.doc", Content: "file2 loren ipsum\n"},
		{Path: "x/b.txt", Content: "file3\n"},
		{Path: "x/y/a.txt", Content: "file4\n"},
		{Path: "x/y/z/a.txt", Content: "file5\n"},
	}
	for i := range gitgrep.MaxResultLimit + 1 {
		files = append(files, git.FastImportFile{Path: "cap" + strconv.Itoa(i) + ".txt", Content: "capped"})
	}
	require.NoError(t, git.ForceFastImport(t.Context(), repo.CodeStorageRepo(), []git.FastImportCommit{{Ref: "refs/heads/master", Files: files}}))
	code_indexer.UpdateRepoIndexer(repo)

	testSearch(t, "/user2/glob/search?q=loren&page=1", []string{"a.txt"})
	doc = testSearch(t, "/user2/glob/search?q=loren+lastMatch", []string{"a.txt"})
	assert.Equal(t, 2, doc.doc.Find(".file-code").Length())
	assert.Equal(t, "5", doc.doc.Find(`.lines-num a[href$="#L5"]`).Text())
	assert.Zero(t, testSearch(t, "/user2/glob/search?q=a.txt", []string{"a.txt"}).doc.Find(".file-code").Length())
	testSearch(t, "/user2/glob/search?q=file3&page=1", []string{"x/b.txt"})
	testSearch(t, "/user2/glob/search?q="+url.QueryEscape("file3 path:x"), []string{"x/b.txt"})
	testSearch(t, "/user2/glob/search?q=file4&page=1&t=match", []string{})
	testSearch(t, "/user2/glob/search?q=file5&page=1&t=match", []string{})

	t.Run("GitGrepIncomplete", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Indexer.RepoIndexerEnabled, false)()
		resp := MakeRequest(t, NewRequest(t, http.MethodGet, "/user2/glob/search?q=capped"), http.StatusOK)
		assert.Contains(t, NewHTMLParser(t, resp.Body).doc.Find(".ui.warning.message").Text(), "Search stopped early")
	})
}

func testSearch(t *testing.T, url string, expected []string) *HTMLDoc {
	t.Helper()
	req := NewRequest(t, "GET", url)
	resp := MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)
	filenames := resultFilenames(doc)
	assert.Equal(t, expected, filenames, "url=%s", url)
	return doc
}

func TestWebSearchRepoCommitStatus(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	branch, err := git_model.GetBranchExisting(t.Context(), repo.ID, repo.DefaultBranch)
	require.NoError(t, err)
	require.NoError(t, commitstatus_service.CreateCommitStatus(t.Context(), repo, owner, branch.CommitID, &git_model.CommitStatus{
		State:     commitstatus.CommitStatusSuccess,
		TargetURL: "https://example.com/ci",
		Context:   "ci",
	}))

	req := NewRequestf(t, "GET", "/repo/search?uid=%d&q=%s", owner.ID, repo.Name)
	resp := loginUser(t, "user4").MakeRequest(t, req, http.StatusOK)
	assert.JSONEq(t, `{
  "ok": true,
  "data": [ {
    "repository": {
      "id": 1, "owner": null, "name": "", "full_name": "user2/repo1", "description": "",
      "empty": false, "private": false, "fork": false, "template": false, "mirror": false, "archived": false, "internal": false,
      "size": 0, "language": "", "languages_url": "", "topics": null, "licenses": null,
      "html_url": "`+setting.AppURL+`user2/repo1", "url": "", "link": "/user2/repo1", "avatar_url": "",
      "ssh_url": "", "clone_url": "", "original_url": "", "website": "",
      "stars_count": 0, "forks_count": 0, "watchers_count": 0, "branch_count": 0,
      "open_issues_count": 0, "open_pr_counter": 0, "release_counter": 0,
      "default_branch": "", "object_format_name": "",
      "created_at": "0001-01-01T00:00:00Z",
      "updated_at": "0001-01-01T00:00:00Z",
      "archived_at": "0001-01-01T00:00:00Z",
      "has_code": false, "has_issues": false, "has_wiki": false, "has_pull_requests": false, "has_projects": false,
      "has_releases": false, "has_packages": false, "has_actions": false,
      "projects_mode": "",
      "ignore_whitespace_conflicts": false,
      "allow_merge_commits": false, "allow_rebase": false, "allow_rebase_explicit": false, "allow_squash_merge": false,
      "allow_fast_forward_only_merge": false, "allow_merge_update": false, "allow_rebase_update": false, "allow_manual_merge": false,
      "autodetect_manual_merge": false, "default_delete_branch_after_merge": false, "default_merge_style": "", "default_update_style": "",
      "squash_merge_commit_message": "",
      "default_allow_maintainer_edit": false,
      "pull_requests_creation_restriction": "",
      "issues_creation_restriction": "",
      "mirror_interval": "", "mirror_updated": "0001-01-01T00:00:00Z", "mirror_last_sync_at": "0001-01-01T00:00:00Z"
    },
    "latest_commit_status":{"state": "success", "target_url": "https://example.com/ci"},
    "locale_latest_commit_status": "Success"
  } ]
}`, resp.Body.String())
}
