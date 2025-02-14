// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"sync"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func TestRepoCommits(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Request repository commits page
	req := NewRequest(t, "GET", "/user2/repo1/commits/branch/master")
	resp := session.MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	commitURL, exists := doc.doc.Find("#commits-table .commit-id-short").Attr("href")
	assert.True(t, exists)
	assert.NotEmpty(t, commitURL)
}

func Test_ReposGitCommitListNotMaster(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)

	// Test getting commits (Page 1)
	req := NewRequestf(t, "GET", "/%s/repo16/commits/branch/master", user.Name)
	resp := session.MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	commits := []string{}
	doc.doc.Find("#commits-table .commit-id-short").Each(func(i int, s *goquery.Selection) {
		commitURL, exists := s.Attr("href")
		assert.True(t, exists)
		assert.NotEmpty(t, commitURL)
		commits = append(commits, path.Base(commitURL))
	})

	assert.Len(t, commits, 3)
	assert.EqualValues(t, "69554a64c1e6030f051e5c3f94bfbd773cd6a324", commits[0])
	assert.EqualValues(t, "27566bd5738fc8b4e3fef3c5e72cce608537bd95", commits[1])
	assert.EqualValues(t, "5099b81332712fe655e34e8dd63574f503f61811", commits[2])

	userNames := []string{}
	doc.doc.Find("#commits-table .author-wrapper").Each(func(i int, s *goquery.Selection) {
		userPath, exists := s.Attr("href")
		assert.True(t, exists)
		assert.NotEmpty(t, userPath)
		userNames = append(userNames, path.Base(userPath))
	})

	assert.Len(t, userNames, 3)
	assert.EqualValues(t, "User2", userNames[0])
	assert.EqualValues(t, "user21", userNames[1])
	assert.EqualValues(t, "User2", userNames[2])
}

func doTestRepoCommitWithStatus(t *testing.T, state string, classes ...string) {
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
	t.Run("CreateStatus", doAPICreateCommitStatus(ctx, path.Base(commitURL), api.CreateStatusOption{
		State:       api.CommitStatusState(state),
		TargetURL:   "http://test.ci/",
		Description: "",
		Context:     "testci",
	}))

	req = NewRequest(t, "GET", "/user2/repo1/commits/branch/master")
	resp = session.MakeRequest(t, req, http.StatusOK)

	doc = NewHTMLParser(t, resp.Body)
	// Check if commit status is displayed in message column (.tippy-target to ignore the tippy trigger)
	sel := doc.doc.Find("#commits-table .message .tippy-target .commit-status")
	assert.Equal(t, 1, sel.Length())
	for _, class := range classes {
		assert.True(t, sel.HasClass(class))
	}

	// By SHA
	req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/commits/"+path.Base(commitURL)+"/statuses")
	reqOne := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/commits/"+path.Base(commitURL)+"/status")
	testRepoCommitsWithStatus(t, session.MakeRequest(t, req, http.StatusOK), session.MakeRequest(t, reqOne, http.StatusOK), state)

	// By short SHA
	req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/commits/"+path.Base(commitURL)[:10]+"/statuses")
	reqOne = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/commits/"+path.Base(commitURL)[:10]+"/status")
	testRepoCommitsWithStatus(t, session.MakeRequest(t, req, http.StatusOK), session.MakeRequest(t, reqOne, http.StatusOK), state)

	// By Ref
	req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/commits/master/statuses")
	reqOne = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/commits/master/status")
	testRepoCommitsWithStatus(t, session.MakeRequest(t, req, http.StatusOK), session.MakeRequest(t, reqOne, http.StatusOK), state)
	req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/commits/v1.1/statuses")
	reqOne = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/commits/v1.1/status")
	testRepoCommitsWithStatus(t, session.MakeRequest(t, req, http.StatusOK), session.MakeRequest(t, reqOne, http.StatusOK), state)
}

func testRepoCommitsWithStatus(t *testing.T, resp, respOne *httptest.ResponseRecorder, state string) {
	var statuses []*api.CommitStatus
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), &statuses))
	var status api.CombinedStatus
	assert.NoError(t, json.Unmarshal(respOne.Body.Bytes(), &status))
	assert.NotNil(t, status)

	if assert.Len(t, statuses, 1) {
		assert.Equal(t, api.CommitStatusState(state), statuses[0].State)
		assert.Equal(t, setting.AppURL+"api/v1/repos/user2/repo1/statuses/65f1bf27bc3bf70f64657658635e66094edbcb4d", statuses[0].URL)
		assert.Equal(t, "http://test.ci/", statuses[0].TargetURL)
		assert.Equal(t, "", statuses[0].Description)
		assert.Equal(t, "testci", statuses[0].Context)

		assert.Len(t, status.Statuses, 1)
		assert.Equal(t, statuses[0], status.Statuses[0])
		assert.Equal(t, "65f1bf27bc3bf70f64657658635e66094edbcb4d", status.SHA)
	}
}

func TestRepoCommitsWithStatusPending(t *testing.T) {
	doTestRepoCommitWithStatus(t, "pending", "octicon-dot-fill", "yellow")
}

func TestRepoCommitsWithStatusSuccess(t *testing.T) {
	doTestRepoCommitWithStatus(t, "success", "octicon-check", "green")
}

func TestRepoCommitsWithStatusError(t *testing.T) {
	doTestRepoCommitWithStatus(t, "error", "gitea-exclamation", "red")
}

func TestRepoCommitsWithStatusFailure(t *testing.T) {
	doTestRepoCommitWithStatus(t, "failure", "octicon-x", "red")
}

func TestRepoCommitsWithStatusWarning(t *testing.T) {
	doTestRepoCommitWithStatus(t, "warning", "gitea-exclamation", "yellow")
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
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(parentT *testing.T, i int) {
			parentT.Run(fmt.Sprintf("ParallelCreateStatus_%d", i), func(t *testing.T) {
				ctx := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository)
				runBody := doAPICreateCommitStatus(ctx, path.Base(commitURL), api.CreateStatusOption{
					State:       api.CommitStatusPending,
					TargetURL:   "http://test.ci/",
					Description: "",
					Context:     "testci",
				})
				runBody(t)
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
	t.Run("CreateStatus", doAPICreateCommitStatus(ctx, path.Base(commitURL), api.CreateStatusOption{
		State:       api.CommitStatusSuccess,
		TargetURL:   "http://test.ci/",
		Description: "",
		Context:     "testci",
	}))

	t.Run("CreateStatus", doAPICreateCommitStatus(ctx, path.Base(commitURL), api.CreateStatusOption{
		State:       api.CommitStatusSuccess,
		TargetURL:   "http://test.ci/",
		Description: "",
		Context:     "other_context",
	}))

	req = NewRequest(t, "GET", "/user2/repo1/commits/branch/master")
	resp = session.MakeRequest(t, req, http.StatusOK)

	doc = NewHTMLParser(t, resp.Body)
	// Check that the data-tippy="commit-statuses" (for trigger) and commit-status (svg) are present
	sel := doc.doc.Find("#commits-table .message [data-tippy=\"commit-statuses\"] .commit-status")
	assert.Equal(t, 1, sel.Length())
}
