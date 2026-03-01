// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/test"
	repo_service "code.gitea.io/gitea/services/repository"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareTag(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/compare/v1.1...master")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	selection := htmlDoc.doc.Find(".ui.dropdown.select-branch")
	// A dropdown for both base and head.
	assert.Lenf(t, selection.Nodes, 2, "The template has changed")

	req = NewRequest(t, "GET", "/user2/repo1/compare/invalid").SetHeader("Accept", "text/html")
	resp = session.MakeRequest(t, req, http.StatusNotFound)
	assert.True(t, test.IsNormalPageCompleted(resp.Body.String()), "expect 404 page not 500")
}

// Compare with inferred default branch (master)
func TestCompareDefault(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/compare/v1.1")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	selection := htmlDoc.doc.Find(".ui.dropdown.select-branch")
	assert.Lenf(t, selection.Nodes, 2, "The template has changed")
}

// Ensure the comparison matches what we expect
func inspectCompare(t *testing.T, htmlDoc *HTMLDoc, diffCount int, diffChanges []string) {
	selection := htmlDoc.doc.Find("#diff-file-boxes").Children()

	assert.Lenf(t, selection.Nodes, diffCount, "Expected %v diffed files, found: %v", diffCount, len(selection.Nodes))

	for _, diffChange := range diffChanges {
		selection = htmlDoc.doc.Find(fmt.Sprintf("[data-new-filename=\"%s\"]", diffChange))
		assert.Lenf(t, selection.Nodes, 1, "Expected 1 match for [data-new-filename=\"%s\"], found: %v", diffChange, len(selection.Nodes))
	}
}

// Git commit graph for repo20
// * 8babce9 (origin/remove-files-b) Add a dummy file
// * b67e43a Delete test.csv and link_hi
// | * cfe3b3c (origin/remove-files-a) Delete test.csv and link_hi
// |/
// * c8e31bc (origin/add-csv) Add test csv file
// * 808038d (HEAD -> master, origin/master, origin/HEAD) Added test links

func TestCompareBranches(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Indirect compare remove-files-b (head) with add-csv (base) branch
	//
	//	'link_hi' and 'test.csv' are deleted, 'test.txt' is added
	req := NewRequest(t, "GET", "/user2/repo20/compare/add-csv...remove-files-b")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	diffCount := 3
	diffChanges := []string{"link_hi", "test.csv", "test.txt"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)

	// Indirect compare remove-files-b (head) with remove-files-a (base) branch
	//
	//	'link_hi' and 'test.csv' are deleted, 'test.txt' is added

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-a...remove-files-b")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffCount = 3
	diffChanges = []string{"link_hi", "test.csv", "test.txt"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)

	// Indirect compare remove-files-a (head) with remove-files-b (base) branch
	//
	//	'link_hi' and 'test.csv' are deleted

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-b...remove-files-a")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffCount = 2
	diffChanges = []string{"link_hi", "test.csv"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)

	// Direct compare remove-files-b (head) with remove-files-a (base) branch
	//
	//	'test.txt' is deleted

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-b..remove-files-a")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffCount = 1
	diffChanges = []string{"test.txt"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)
}

func TestCompareCodeExpand(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		repo, err := repo_service.CreateRepositoryDirectly(t.Context(), user1, user1, repo_service.CreateRepoOptions{
			Name:          "test_blob_excerpt",
			Readme:        "Default",
			AutoInit:      true,
			DefaultBranch: "main",
		}, true)
		assert.NoError(t, err)

		session := loginUser(t, user1.Name)
		testEditFile(t, session, user1.Name, repo.Name, "main", "README.md", strings.Repeat("a\n", 30))

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session = loginUser(t, user2.Name)
		testRepoFork(t, session, user1.Name, repo.Name, user2.Name, "test_blob_excerpt-fork", "")
		testCreateBranch(t, session, user2.Name, "test_blob_excerpt-fork", "branch/main", "forked-branch", http.StatusSeeOther)
		testEditFile(t, session, user2.Name, "test_blob_excerpt-fork", "forked-branch", "README.md", strings.Repeat("a\n", 15)+"CHANGED\n"+strings.Repeat("a\n", 15))

		req := NewRequest(t, "GET", "/user1/test_blob_excerpt/compare/main...user2/test_blob_excerpt-fork:forked-branch")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		els := htmlDoc.Find(`button.code-expander-button[data-url]`)

		// all the links in the comparison should be to the forked repo&branch
		assert.NotZero(t, els.Length())
		for i := 0; i < els.Length(); i++ {
			link := els.Eq(i).AttrOr("data-url", "")
			assert.True(t, strings.HasPrefix(link, "/user2/test_blob_excerpt-fork/blob_excerpt/"))
		}
	})
}

func TestBlobExcerptSingleAndBatch(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		repo, err := repo_service.CreateRepositoryDirectly(t.Context(), user1, user1, repo_service.CreateRepoOptions{
			Name:          "test_blob_excerpt_batch",
			Readme:        "Default",
			AutoInit:      true,
			DefaultBranch: "main",
		}, true)
		require.NoError(t, err)

		session := loginUser(t, user1.Name)

		// Create a file with 50 lines so the diff has multiple collapsed sections
		lines := make([]string, 50)
		for i := range lines {
			lines[i] = fmt.Sprintf("line %d", i+1)
		}
		testEditFile(t, session, user1.Name, repo.Name, "main", "README.md", strings.Join(lines, "\n")+"\n")

		// Create a branch and change a line in the middle to produce two expander gaps
		testEditFileToNewBranch(t, session, user1.Name, repo.Name, "main", "excerpt-branch", "README.md",
			func() string {
				modified := make([]string, 50)
				copy(modified, lines)
				modified[24] = "CHANGED line 25"
				return strings.Join(modified, "\n") + "\n"
			}(),
		)

		// Load the compare page
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/compare/main...excerpt-branch", user1.Name, repo.Name))
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		els := htmlDoc.Find(`button.code-expander-button[data-url]`)

		// We need at least 2 expander buttons to test batch mode
		require.GreaterOrEqual(t, els.Length(), 2, "expected at least 2 expander buttons")

		// Deduplicate by anchor param to get one URL per collapsed section
		// (updown rows have two buttons with the same anchor but different directions)
		seen := map[string]bool{}
		var expanderURLs []string
		for i := range els.Length() {
			link := els.Eq(i).AttrOr("data-url", "")
			parsed, err := url.Parse(link)
			require.NoError(t, err)
			anchor := parsed.Query().Get("anchor")
			if !seen[anchor] {
				seen[anchor] = true
				expanderURLs = append(expanderURLs, link)
			}
		}
		require.GreaterOrEqual(t, len(expanderURLs), 2, "expected at least 2 unique expander sections")

		t.Run("SingleFetch", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			// Rewrite direction to "full" as the frontend does for expand-all
			singleURL := strings.Replace(expanderURLs[0], "direction=down", "direction=full", 1)
			singleURL = strings.Replace(singleURL, "direction=up", "direction=full", 1)
			req := NewRequest(t, "GET", singleURL)
			resp := session.MakeRequest(t, req, http.StatusOK)
			body := resp.Body.String()
			// Single mode returns HTML directly, should contain diff table rows
			assert.Contains(t, body, `class="lines-`)
			assert.NotContains(t, body, `[`) // should not be JSON
		})

		t.Run("BatchFetch", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Parse per-gap params from each expander URL and join with commas
			paramKeys := []string{"last_left", "last_right", "left", "right", "left_hunk_size", "right_hunk_size"}
			batchValues := make(map[string][]string)
			var basePath string
			var sharedParams url.Values

			for i, expanderURL := range expanderURLs {
				parsed, err := url.Parse(expanderURL)
				require.NoError(t, err)
				if i == 0 {
					basePath = parsed.Path
					sharedParams = parsed.Query()
				}
				q := parsed.Query()
				for _, key := range paramKeys {
					batchValues[key] = append(batchValues[key], q.Get(key))
				}
			}

			// Build batch URL
			batchParams := url.Values{}
			for _, key := range paramKeys {
				batchParams.Set(key, strings.Join(batchValues[key], ","))
			}
			for _, key := range []string{"path", "filelang", "style"} {
				if v := sharedParams.Get(key); v != "" {
					batchParams.Set(key, v)
				}
			}
			batchParams.Set("direction", "full")
			batchURL := basePath + "?" + batchParams.Encode()

			req := NewRequest(t, "GET", batchURL)
			resp := session.MakeRequest(t, req, http.StatusOK)

			// Batch mode returns a JSON array of HTML strings
			var htmlArray []string
			err := json.Unmarshal(resp.Body.Bytes(), &htmlArray)
			require.NoError(t, err, "response should be valid JSON string array")
			assert.Len(t, htmlArray, len(expanderURLs))

			for i, html := range htmlArray {
				assert.Contains(t, html, `class="lines-`, "batch result %d should contain diff HTML", i)
			}
		})

		t.Run("BatchFetchMismatchedParams", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Build a batch URL with mismatched param lengths — should return 400
			parsed, err := url.Parse(expanderURLs[0])
			require.NoError(t, err)
			q := parsed.Query()
			q.Set("last_left", q.Get("last_left")+",0") // 2 values
			// other params remain with 1 value
			badURL := parsed.Path + "?" + q.Encode()
			req := NewRequest(t, "GET", badURL)
			session.MakeRequest(t, req, http.StatusBadRequest)
		})
	})
}
