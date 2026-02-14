// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"path"
	"sync"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/modules/commitstatus"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoCommits(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	t.Run("CommitList", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo16/commits/branch/master")
		resp := session.MakeRequest(t, req, http.StatusOK)

		var commits, userHrefs []string
		doc := NewHTMLParser(t, resp.Body)
		doc.doc.Find("#commits-table .commit-id-short").Each(func(i int, s *goquery.Selection) {
			commits = append(commits, path.Base(s.AttrOr("href", "")))
		})
		doc.doc.Find("#commits-table .author-wrapper").Each(func(i int, s *goquery.Selection) {
			userHrefs = append(userHrefs, s.AttrOr("href", ""))
		})
		assert.Equal(t, []string{"69554a64c1e6030f051e5c3f94bfbd773cd6a324", "27566bd5738fc8b4e3fef3c5e72cce608537bd95", "5099b81332712fe655e34e8dd63574f503f61811"}, commits)
		assert.Equal(t, []string{"/user2", "/user21", "/user2"}, userHrefs)
	})

	t.Run("LastCommit", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo16")
		resp := session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)
		commitHref := doc.doc.Find(".latest-commit .commit-id-short").AttrOr("href", "")
		authorHref := doc.doc.Find(".latest-commit .author-wrapper").AttrOr("href", "")
		assert.Equal(t, "/user2/repo16/commit/69554a64c1e6030f051e5c3f94bfbd773cd6a324", commitHref)
		assert.Equal(t, "/user2", authorHref)
	})

	t.Run("CommitListNonExistingCommiter", func(t *testing.T) {
		// check the commit list for a repository with no gitea user
		// * commit 985f0301dba5e7b34be866819cd15ad3d8f508ee (branch2)
		// * Author: 6543 <6543@obermui.de>
		req := NewRequest(t, "GET", "/user2/repo1/commits/branch/branch2")
		resp := session.MakeRequest(t, req, http.StatusOK)

		doc := NewHTMLParser(t, resp.Body)
		commitHref := doc.doc.Find("#commits-table tr:first-child .commit-id-short").AttrOr("href", "")
		assert.Equal(t, "/user2/repo1/commit/985f0301dba5e7b34be866819cd15ad3d8f508ee", commitHref)
		authorElem := doc.doc.Find("#commits-table tr:first-child .author-wrapper")
		assert.Equal(t, "6543", authorElem.Text())
		assert.Equal(t, "span", authorElem.Nodes[0].Data)
	})

	t.Run("LastCommitNonExistingCommiter", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo1/src/branch/branch2")
		resp := session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)
		commitHref := doc.doc.Find(".latest-commit .commit-id-short").AttrOr("href", "")
		assert.Equal(t, "/user2/repo1/commit/985f0301dba5e7b34be866819cd15ad3d8f508ee", commitHref)
		authorElem := doc.doc.Find(".latest-commit .author-wrapper")
		assert.Equal(t, "6543", authorElem.Text())
		assert.Equal(t, "span", authorElem.Nodes[0].Data)
	})
}

func TestRepoCommitsWithStatus(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	ctx := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository)

	requestCommitStatuses := func(t *testing.T, linkList, linkCombined string) (statuses []*api.CommitStatus, status api.CombinedStatus) {
		assert.NoError(t, json.Unmarshal(session.MakeRequest(t, NewRequest(t, "GET", linkList), http.StatusOK).Body.Bytes(), &statuses))
		assert.NoError(t, json.Unmarshal(session.MakeRequest(t, NewRequest(t, "GET", linkCombined), http.StatusOK).Body.Bytes(), &status))
		return statuses, status
	}

	testRefMaster := func(t *testing.T, state commitstatus.CommitStatusState, classes ...string) {
		_ = db.TruncateBeans(t.Context(), &git_model.CommitStatus{})

		// Request repository commits page
		req := NewRequest(t, "GET", "/user2/repo1/commits/branch/master")
		resp := session.MakeRequest(t, req, http.StatusOK)

		doc := NewHTMLParser(t, resp.Body)
		// Get first commit URL
		commitURL, _ := doc.doc.Find("#commits-table .commit-id-short").Attr("href")
		require.NotEmpty(t, commitURL)
		commitID := path.Base(commitURL)

		// Call API to add status for commit
		doAPICreateCommitStatusTest(ctx, path.Base(commitURL), state, "testci")(t)

		req = NewRequest(t, "GET", "/user2/repo1/commits/branch/master")
		resp = session.MakeRequest(t, req, http.StatusOK)

		doc = NewHTMLParser(t, resp.Body)
		// Check if commit status is displayed in message column (.tippy-target to ignore the tippy trigger)
		sel := doc.doc.Find("#commits-table .message .tippy-target .commit-status")
		assert.Equal(t, 1, sel.Length())
		for _, class := range classes {
			assert.True(t, sel.HasClass(class))
		}

		testRepoCommitsWithStatus := func(t *testing.T, linkList, linkCombined string, state commitstatus.CommitStatusState) {
			statuses, status := requestCommitStatuses(t, linkList, linkCombined)
			require.Len(t, statuses, 1)
			require.NotNil(t, status)

			assert.Equal(t, state, statuses[0].State)
			assert.Equal(t, setting.AppURL+"api/v1/repos/user2/repo1/statuses/"+commitID, statuses[0].URL)
			assert.Equal(t, "http://test.ci/", statuses[0].TargetURL)
			assert.Empty(t, statuses[0].Description)
			assert.Equal(t, "testci", statuses[0].Context)

			assert.Len(t, status.Statuses, 1)
			assert.Equal(t, statuses[0], status.Statuses[0])
			assert.Equal(t, commitID, status.SHA)
		}
		// By SHA
		testRepoCommitsWithStatus(t, "/api/v1/repos/user2/repo1/commits/"+commitID+"/statuses", "/api/v1/repos/user2/repo1/commits/"+commitID+"/status", state)
		// By short SHA
		testRepoCommitsWithStatus(t, "/api/v1/repos/user2/repo1/commits/"+commitID[:7]+"/statuses", "/api/v1/repos/user2/repo1/commits/"+commitID[:7]+"/status", state)
		// By Ref
		testRepoCommitsWithStatus(t, "/api/v1/repos/user2/repo1/commits/master/statuses", "/api/v1/repos/user2/repo1/commits/master/status", state)
		// Tag "v1.1" points to master
		testRepoCommitsWithStatus(t, "/api/v1/repos/user2/repo1/commits/v1.1/statuses", "/api/v1/repos/user2/repo1/commits/v1.1/status", state)
	}

	t.Run("pending", func(t *testing.T) { testRefMaster(t, "pending", "octicon-dot-fill", "yellow") })
	t.Run("success", func(t *testing.T) { testRefMaster(t, "success", "octicon-check", "green") })
	t.Run("error", func(t *testing.T) { testRefMaster(t, "error", "gitea-exclamation", "red") })
	t.Run("failure", func(t *testing.T) { testRefMaster(t, "failure", "octicon-x", "red") })
	t.Run("warning", func(t *testing.T) { testRefMaster(t, "warning", "gitea-exclamation", "yellow") })
	t.Run("BranchWithSlash", func(t *testing.T) {
		_ = db.TruncateBeans(t.Context(), &git_model.CommitStatus{})

		linkList, linkCombined := "/api/v1/repos/user2/repo1/commits/feature%2F1/statuses", "/api/v1/repos/user2/repo1/commits/feature/1/status"
		statuses, status := requestCommitStatuses(t, linkList, linkCombined)
		assert.Empty(t, statuses)
		assert.Empty(t, status.Statuses)
		doAPICreateCommitStatusTest(ctx, "feature/1", commitstatus.CommitStatusSuccess, "testci")(t)
		statuses, status = requestCommitStatuses(t, linkList, linkCombined)
		assert.NotEmpty(t, statuses)
		assert.NotEmpty(t, status.Statuses)
	})
}

func TestRepoCommitsStatusParallel(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Request repository commits page
	req := NewRequest(t, "GET", "/user2/repo1/commits/branch/master")
	resp := session.MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	// Get first commit URL
	commitURL, exists := doc.doc.Find("#commits-table .commit-id-short").Attr("href")
	assert.True(t, exists)
	assert.NotEmpty(t, commitURL)

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(parentT *testing.T, i int) {
			parentT.Run(fmt.Sprintf("ParallelCreateStatus_%d", i), func(t *testing.T) {
				ctx := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository)
				doAPICreateCommitStatusTest(ctx, path.Base(commitURL), commitstatus.CommitStatusPending, "testci")(t)
				wg.Done()
			})
		}(t, i)
	}
	wg.Wait()
}

func TestRepoCommitsStatusMultiple(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Request repository commits page
	req := NewRequest(t, "GET", "/user2/repo1/commits/branch/master")
	resp := session.MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	// Get first commit URL
	commitURL, exists := doc.doc.Find("#commits-table .commit-id-short").Attr("href")
	assert.True(t, exists)
	assert.NotEmpty(t, commitURL)

	// Call API to add status for commit
	ctx := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository)
	t.Run("CreateStatus", doAPICreateCommitStatusTest(ctx, path.Base(commitURL), commitstatus.CommitStatusSuccess, "testci"))
	t.Run("CreateStatus", doAPICreateCommitStatusTest(ctx, path.Base(commitURL), commitstatus.CommitStatusSuccess, "other_context"))
	req = NewRequest(t, "GET", "/user2/repo1/commits/branch/master")
	resp = session.MakeRequest(t, req, http.StatusOK)

	doc = NewHTMLParser(t, resp.Body)
	// Check that the data-global-init="initCommitStatuses" (for trigger) and commit-status (svg) are present
	sel := doc.doc.Find(`#commits-table .message [data-global-init="initCommitStatuses"] .commit-status`)
	assert.Equal(t, 1, sel.Length())
}
