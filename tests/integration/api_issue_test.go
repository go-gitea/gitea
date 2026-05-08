// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIIssue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("ListIssues", testAPIListIssues)
	t.Run("ListIssuesPublicOnly", testAPIListIssuesPublicOnly)
	t.Run("SearchIssues", testAPISearchIssues)
	t.Run("SearchIssuesWithLabels", testAPISearchIssuesWithLabels)
	t.Run("EditIssue", testAPIEditIssue)
	t.Run("IssueContentVersion", testAPIIssueContentVersion)
	t.Run("CreateIssue", testAPICreateIssue)
	t.Run("CreateIssueParallel", testAPICreateIssueParallel)
	t.Run("IssueProjects", testAPIIssueProjects)
}

func testAPIListIssues(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadIssue)
	link, _ := url.Parse(fmt.Sprintf("/api/v1/repos/%s/%s/issues", owner.Name, repo.Name))

	link.RawQuery = url.Values{"token": {token}, "state": {"all"}}.Encode()
	resp := MakeRequest(t, NewRequest(t, "GET", link.String()), http.StatusOK)
	apiIssues := DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, unittest.GetCount(t, &issues_model.Issue{RepoID: repo.ID}))
	for _, apiIssue := range apiIssues {
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: apiIssue.ID, RepoID: repo.ID})
	}

	// test milestone filter
	link.RawQuery = url.Values{"token": {token}, "state": {"all"}, "type": {"all"}, "milestones": {"ignore,milestone1,3,4"}}.Encode()
	resp = MakeRequest(t, NewRequest(t, "GET", link.String()), http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	if assert.Len(t, apiIssues, 2) {
		assert.EqualValues(t, 3, apiIssues[0].Milestone.ID)
		assert.EqualValues(t, 1, apiIssues[1].Milestone.ID)
	}

	link.RawQuery = url.Values{"token": {token}, "state": {"all"}, "created_by": {"user2"}}.Encode()
	resp = MakeRequest(t, NewRequest(t, "GET", link.String()), http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	if assert.Len(t, apiIssues, 1) {
		assert.EqualValues(t, 5, apiIssues[0].ID)
	}

	link.RawQuery = url.Values{"token": {token}, "state": {"all"}, "assigned_by": {"user1"}}.Encode()
	resp = MakeRequest(t, NewRequest(t, "GET", link.String()), http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	if assert.Len(t, apiIssues, 1) {
		assert.EqualValues(t, 1, apiIssues[0].ID)
	}

	link.RawQuery = url.Values{"token": {token}, "state": {"all"}, "mentioned_by": {"user4"}}.Encode()
	resp = MakeRequest(t, NewRequest(t, "GET", link.String()), http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	if assert.Len(t, apiIssues, 1) {
		assert.EqualValues(t, 1, apiIssues[0].ID)
	}
}

func testAPIListIssuesPublicOnly(t *testing.T) {
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo1.OwnerID})

	session := loginUser(t, owner1.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadIssue)
	link, _ := url.Parse(fmt.Sprintf("/api/v1/repos/%s/%s/issues", owner1.Name, repo1.Name))
	link.RawQuery = url.Values{"state": {"all"}}.Encode()
	req := NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	owner2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo2.OwnerID})

	session = loginUser(t, owner2.Name)
	token = getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadIssue)
	link, _ = url.Parse(fmt.Sprintf("/api/v1/repos/%s/%s/issues", owner2.Name, repo2.Name))
	link.RawQuery = url.Values{"state": {"all"}}.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	publicOnlyToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadIssue, auth_model.AccessTokenScopePublicOnly)
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(publicOnlyToken)
	MakeRequest(t, req, http.StatusForbidden)
}

func testAPICreateIssue(t *testing.T) {
	const body, title = "apiTestBody", "apiTestTitle"

	repoBefore := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repoBefore.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues", owner.Name, repoBefore.Name)
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
		Body:     body,
		Title:    title,
		Assignee: owner.Name,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	apiIssue := DecodeJSON(t, resp, &api.Issue{})
	assert.Equal(t, body, apiIssue.Body)
	assert.Equal(t, title, apiIssue.Title)

	unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{
		RepoID:     repoBefore.ID,
		AssigneeID: owner.ID,
		Content:    body,
		Title:      title,
	})

	repoAfter := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Equal(t, repoBefore.NumIssues+1, repoAfter.NumIssues)
	assert.Equal(t, repoBefore.NumClosedIssues, repoAfter.NumClosedIssues)

	user34 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34})
	req = NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
		Title: title,
	}).AddTokenAuth(getUserToken(t, user34.Name, auth_model.AccessTokenScopeWriteIssue))
	MakeRequest(t, req, http.StatusForbidden)
}

func testAPICreateIssueParallel(t *testing.T) {
	// HINT: There seems to be a bug in github.com/mattn/go-sqlite3 with sqlite_unlock_notify, when doing concurrent writes to the same database,
	// some requests may get stuck in "go-sqlite3.(*SQLiteRows).Next", "go-sqlite3.(*SQLiteStmt).exec" and "go-sqlite3.unlock_notify_wait",
	// because the "unlock_notify_wait" never returns and the internal lock never gets released.
	//
	// The trigger is: a previous test created issues and made the real issue indexer queue start processing, then this test does concurrent writing.
	// Adding this "Sleep" makes go-sqlite3 "finish" some internal operations before concurrent writes and then won't get stuck.
	// To reproduce: make a new test run these 2 tests enough times:
	// > func testBug() { for i := 0; i < 100; i++ { testAPICreateIssue(t); testAPICreateIssueParallel(t) } }
	// Usually the test gets stuck in fewer than 10 iterations without this "sleep".
	time.Sleep(100 * time.Millisecond)

	const body, title = "apiTestBody", "apiTestTitle"

	repoBefore := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repoBefore.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues", owner.Name, repoBefore.Name)

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(parentT *testing.T, i int) {
			parentT.Run(fmt.Sprintf("ParallelCreateIssue_%d", i), func(t *testing.T) {
				newTitle := title + strconv.Itoa(i)
				newBody := body + strconv.Itoa(i)
				req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
					Body:     newBody,
					Title:    newTitle,
					Assignee: owner.Name,
				}).AddTokenAuth(token)
				resp := MakeRequest(t, req, http.StatusCreated)
				apiIssue := DecodeJSON(t, resp, &api.Issue{})
				assert.Equal(t, newBody, apiIssue.Body)
				assert.Equal(t, newTitle, apiIssue.Title)

				unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{
					RepoID:     repoBefore.ID,
					AssigneeID: owner.ID,
					Content:    newBody,
					Title:      newTitle,
				})

				wg.Done()
			})
		}(t, i)
	}
	wg.Wait()
}

func testAPIEditIssue(t *testing.T) {
	issueBefore := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 10})
	repoBefore := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issueBefore.RepoID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repoBefore.OwnerID})
	assert.NoError(t, issueBefore.LoadAttributes(t.Context()))
	assert.Equal(t, int64(1019307200), int64(issueBefore.DeadlineUnix))
	assert.Equal(t, api.StateOpen, issueBefore.State())

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	// update values of issue
	issueState := "closed"
	removeDeadline := true
	milestone := int64(4)
	body := "new content!"
	title := "new title from api set"

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", owner.Name, repoBefore.Name, issueBefore.Index)
	req := NewRequestWithJSON(t, "PATCH", urlStr, api.EditIssueOption{
		State:          &issueState,
		RemoveDeadline: &removeDeadline,
		Milestone:      &milestone,
		Body:           &body,
		Title:          title,

		// ToDo change more
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	apiIssue := DecodeJSON(t, resp, &api.Issue{})

	issueAfter := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 10})
	repoAfter := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issueBefore.RepoID})

	// check comment history
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: issueAfter.ID, OldTitle: issueBefore.Title, NewTitle: title})
	unittest.AssertExistsAndLoadBean(t, &issues_model.ContentHistory{IssueID: issueAfter.ID, ContentText: body, IsFirstCreated: false})

	// check deleted user
	assert.Equal(t, int64(500), issueAfter.PosterID)
	assert.NoError(t, issueAfter.LoadAttributes(t.Context()))
	assert.Equal(t, int64(-1), issueAfter.PosterID)
	assert.Equal(t, int64(-1), issueBefore.PosterID)
	assert.Equal(t, int64(-1), apiIssue.Poster.ID)

	// check repo change
	assert.Equal(t, repoBefore.NumClosedIssues+1, repoAfter.NumClosedIssues)

	// API response
	assert.Equal(t, api.StateClosed, apiIssue.State)
	assert.Equal(t, milestone, apiIssue.Milestone.ID)
	assert.Equal(t, body, apiIssue.Body)
	assert.Nil(t, apiIssue.Deadline)
	assert.Equal(t, title, apiIssue.Title)

	// in database
	assert.Equal(t, api.StateClosed, issueAfter.State())
	assert.Equal(t, milestone, issueAfter.MilestoneID)
	assert.Equal(t, int64(0), int64(issueAfter.DeadlineUnix))
	assert.Equal(t, body, issueAfter.Content)
	assert.Equal(t, title, issueAfter.Title)
}

func testAPISearchIssues(t *testing.T) {
	defer test.MockVariableValue(&setting.API.DefaultPagingNum, 20)()
	expectedIssueCount := 20 // 20 is from the fixtures

	link, _ := url.Parse("/api/v1/repos/issues/search")
	token := getUserToken(t, "user1", auth_model.AccessTokenScopeReadIssue)
	query := url.Values{}
	var apiIssues []*api.Issue

	link.RawQuery = query.Encode()
	req := NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, expectedIssueCount)

	publicOnlyToken := getUserToken(t, "user1", auth_model.AccessTokenScopeReadIssue, auth_model.AccessTokenScopePublicOnly)
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(publicOnlyToken)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 15) // 15 public issues

	since := "2000-01-01T00:50:01+00:00" // 946687801
	before := time.Unix(999307200, 0).Format(time.RFC3339)
	query.Add("since", since)
	query.Add("before", before)
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 11)
	query.Del("since")
	query.Del("before")

	query.Add("state", "closed")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 2)

	query.Set("state", "all")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Equal(t, "22", resp.Header().Get("X-Total-Count"))
	assert.Len(t, apiIssues, 20)

	query.Add("limit", "10")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Equal(t, "22", resp.Header().Get("X-Total-Count"))
	assert.Len(t, apiIssues, 10)

	query = url.Values{"assigned": {"true"}, "state": {"all"}}
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 2)

	query = url.Values{"milestones": {"milestone1"}, "state": {"all"}}
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 1)

	query = url.Values{"milestones": {"milestone1,milestone3"}, "state": {"all"}}
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 2)

	query = url.Values{"owner": {"user2"}} // user
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 8)

	query = url.Values{"owner": {"org3"}} // organization
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 5)

	query = url.Values{"owner": {"org3"}, "team": {"team1"}} // organization + team
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 2)

	query = url.Values{"created": {"1"}} // issues created by the auth user
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 5)

	query = url.Values{"created": {"1"}, "type": {"pulls"}} // prs created by the auth user
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 3)

	query = url.Values{"created_by": {"user2"}} // issues created by the user2
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 9)

	query = url.Values{"created_by": {"user2"}, "type": {"pulls"}} // prs created by user2
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 3)
}

func testAPISearchIssuesWithLabels(t *testing.T) {
	// as this API was used in the frontend, it uses UI page size
	expectedIssueCount := min(20, setting.UI.IssuePagingNum) // 20 is from the fixtures

	link, _ := url.Parse("/api/v1/repos/issues/search")
	token := getUserToken(t, "user1", auth_model.AccessTokenScopeReadIssue)
	query := url.Values{}
	var apiIssues []*api.Issue

	link.RawQuery = query.Encode()
	req := NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, expectedIssueCount)

	query.Add("labels", "label1")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 2)

	// multiple labels
	query.Set("labels", "label1,label2")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 2)

	// an org label
	query.Set("labels", "orglabel4")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 1)

	// org and repo label
	query.Set("labels", "label2,orglabel4")
	query.Add("state", "all")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 2)

	// org and repo label which share the same issue
	query.Set("labels", "label1,orglabel4")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiIssues = DecodeJSON(t, resp, []*api.Issue{})
	assert.Len(t, apiIssues, 2)
}

func testAPIIssueContentVersion(t *testing.T) {
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 10})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", owner.Name, repo.Name, issue.Index)

	t.Run("ResponseIncludesContentVersion", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", urlStr).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		apiIssue := DecodeJSON(t, resp, &api.Issue{})
		assert.GreaterOrEqual(t, apiIssue.ContentVersion, 0)
	})

	t.Run("EditWithCorrectVersion", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", urlStr).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		before := DecodeJSON(t, resp, &api.Issue{})
		req = NewRequestWithJSON(t, "PATCH", urlStr, api.EditIssueOption{
			Body:           new("updated body with correct version"),
			ContentVersion: new(before.ContentVersion),
		}).AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusCreated)
		after := DecodeJSON(t, resp, &api.Issue{})
		assert.Equal(t, "updated body with correct version", after.Body)
		assert.Greater(t, after.ContentVersion, before.ContentVersion)
	})

	t.Run("EditWithWrongVersion", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequestWithJSON(t, "PATCH", urlStr, api.EditIssueOption{
			Body:           new("should fail"),
			ContentVersion: new(99999),
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusConflict)
	})

	t.Run("EditWithoutVersion", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequestWithJSON(t, "PATCH", urlStr, api.EditIssueOption{
			Body: new("edit without version succeeds"),
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
	})
}

func testAPIIssueProjects(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues", owner.Name, repo.Name)

	// Create issue with a project
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
		Title:    "issue with project",
		Body:     "test body",
		Projects: []int64{1},
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	var apiIssue api.Issue
	DecodeJSON(t, resp, &apiIssue)
	assert.Len(t, apiIssue.Projects, 1)
	assert.EqualValues(t, 1, apiIssue.Projects[0].ID)

	// Get issue should include projects
	req = NewRequest(t, "GET", fmt.Sprintf("%s/%d", urlStr, apiIssue.Index)).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssue)
	assert.Len(t, apiIssue.Projects, 1)
	assert.EqualValues(t, 1, apiIssue.Projects[0].ID)

	// Edit issue to remove projects
	emptyProjects := []int64{}
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("%s/%d", urlStr, apiIssue.Index), &api.EditIssueOption{
		Projects: &emptyProjects,
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, &apiIssue)
	assert.Empty(t, apiIssue.Projects)

	// Edit issue to add project back
	projects := []int64{1}
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("%s/%d", urlStr, apiIssue.Index), &api.EditIssueOption{
		Projects: &projects,
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, &apiIssue)
	assert.Len(t, apiIssue.Projects, 1)
	assert.EqualValues(t, 1, apiIssue.Projects[0].ID)

	// Test invalid project ID
	req = NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
		Title:    "issue with invalid project",
		Body:     "test body",
		Projects: []int64{99999},
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusBadRequest)

	// Test project from different repo (project 2 is for repo 3)
	req = NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
		Title:    "issue with inaccessible project",
		Body:     "test body",
		Projects: []int64{2},
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusBadRequest)
}
