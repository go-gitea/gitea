// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"encoding/xml"
	"fmt"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	activities_model "gitea.dev/models/activities"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	issue_indexer "gitea.dev/modules/indexer/issues"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
	"gitea.dev/services/mailer/incoming"
	incoming_payload "gitea.dev/services/mailer/incoming/payload"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/contexts"
)

func TestConfidentialIssueCreateAndToggle(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	writer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	require.NoError(t, repo_service.AddOrUpdateCollaborator(t.Context(), repo, writer, perm.AccessModeWrite))
	poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})

	newIssue := func(t *testing.T, confidential bool) *issues_model.Issue {
		issue := &issues_model.Issue{RepoID: repo.ID, Repo: repo, Index: 9000 + int64(unittest.GetCount(t, &issues_model.Issue{RepoID: repo.ID})), PosterID: poster.ID, Poster: poster, Title: "toggle test", IsConfidential: confidential}
		require.NoError(t, db.Insert(t.Context(), issue))
		return issue
	}
	commentCount := func(t *testing.T, issue *issues_model.Issue, commentType issues_model.CommentType) int {
		return unittest.GetCount(t, &issues_model.Comment{IssueID: issue.ID, Type: commentType})
	}
	toggleURL := func(issue *issues_model.Issue) string {
		return fmt.Sprintf("/user2/repo1/issues/%d/confidential", issue.Index)
	}
	post := func(t *testing.T, user, link, value string, status int) *httptest.ResponseRecorder {
		req := NewRequestWithValues(t, "POST", link, map[string]string{"confidential": value})
		return loginUser(t, user).MakeRequest(t, req, status)
	}
	type errorBody struct {
		ErrorMessage string `json:"errorMessage"`
	}

	t.Run("writer toggles", func(t *testing.T) {
		issue := newIssue(t, false)
		resp := post(t, "user4", toggleURL(issue), "true", http.StatusOK)
		assert.Equal(t, fmt.Sprintf("/user2/repo1/issues/%d", issue.Index), test.RedirectURL(resp))
		assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID}).IsConfidential)
		assert.Equal(t, 1, commentCount(t, issue, issues_model.CommentTypeMakeConfidential))

		post(t, "user4", toggleURL(issue), "true", http.StatusOK)
		assert.Equal(t, 1, commentCount(t, issue, issues_model.CommentTypeMakeConfidential), "no-op writes no event")

		htmlDoc := NewHTMLParser(t, loginUser(t, "user4").MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/user2/repo1/issues/%d", issue.Index)), http.StatusOK).Body)
		assert.Contains(t, htmlDoc.Find(".issue-title-meta .ui.warning.label").Text(), "Confidential")
		assert.Contains(t, htmlDoc.Find(".timeline-item.event").Text(), "made this issue confidential")
		assert.Equal(t, 1, htmlDoc.Find(`form[action$="/confidential"] input[name=confidential][value=false]`).Length())

		post(t, "user4", toggleURL(issue), "false", http.StatusOK)
		assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID}).IsConfidential)
		assert.Equal(t, 1, commentCount(t, issue, issues_model.CommentTypeMakePublic))
		htmlDoc = NewHTMLParser(t, loginUser(t, "user4").MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/user2/repo1/issues/%d", issue.Index)), http.StatusOK).Body)
		assert.Contains(t, htmlDoc.Find(".timeline-item.event").Text(), "made this issue public")
	})

	t.Run("poster without write access is forbidden and has no toggle", func(t *testing.T) {
		issue := newIssue(t, false)
		resp := post(t, "user10", toggleURL(issue), "true", http.StatusForbidden)
		assert.Equal(t, "You do not have permission to change the confidential setting of this issue.", DecodeJSON(t, resp, &errorBody{}).ErrorMessage)
		assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID}).IsConfidential)

		htmlDoc := NewHTMLParser(t, loginUser(t, "user10").MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/user2/repo1/issues/%d", issue.Index)), http.StatusOK).Body)
		assert.Zero(t, htmlDoc.Find("#sidebar-confidential").Length())
	})

	t.Run("hidden viewer sees not found", func(t *testing.T) {
		issue := newIssue(t, true)
		post(t, "user5", toggleURL(issue), "false", http.StatusNotFound)
		assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID}).IsConfidential)
	})

	t.Run("pull request", func(t *testing.T) {
		resp := post(t, "user2", "/user2/repo1/pulls/2/confidential", "true", http.StatusUnprocessableEntity)
		assert.Equal(t, "Pull requests cannot be confidential.", DecodeJSON(t, resp, &errorBody{}).ErrorMessage)
		post(t, "user2", "/user2/repo1/pulls/2/confidential", "false", http.StatusOK)
	})

	t.Run("invalid value", func(t *testing.T) {
		issue := newIssue(t, false)
		resp := post(t, "user4", toggleURL(issue), "banana", http.StatusBadRequest)
		assert.Equal(t, "Invalid value for Confidential.", DecodeJSON(t, resp, &errorBody{}).ErrorMessage)
		assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID}).IsConfidential)
	})

	t.Run("create", func(t *testing.T) {
		session := loginUser(t, "user2")
		doc := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/issues/new"), http.StatusOK).Body)
		assert.Equal(t, "Confidential", strings.TrimSpace(doc.Find(`label[for="issue-confidential"]`).Text()))
		assert.Equal(t, 1, doc.Find(`input#issue-confidential[type=checkbox][name=confidential]`).Length())
		assert.Contains(t, doc.Find("#issue-confidential-help").Text(), "Only you, assignees and people with write access will see this issue.")

		resp := session.MakeRequest(t, NewRequestWithValues(t, "POST", "/user2/repo1/issues/new", map[string]string{"title": "secret title", "confidential": "true"}), http.StatusOK)
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID, Title: "secret title"})
		assert.True(t, issue.IsConfidential)
		assert.Zero(t, commentCount(t, issue, issues_model.CommentTypeMakeConfidential))
		doc = NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", test.RedirectURL(resp)), http.StatusOK).Body)
		assert.Contains(t, doc.Find(".issue-title-meta .ui.warning.label").Text(), "Confidential")

		session.MakeRequest(t, NewRequestWithValues(t, "POST", "/user2/repo1/issues/new", map[string]string{"title": "public title"}), http.StatusOK)
		assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID, Title: "public title"}).IsConfidential)

		resp = session.MakeRequest(t, NewRequestWithValues(t, "POST", "/user2/repo1/issues/new", map[string]string{"title": "banana title", "confidential": "banana"}), http.StatusBadRequest)
		assert.Equal(t, "Invalid value for Confidential.", DecodeJSON(t, resp, &errorBody{}).ErrorMessage)
		unittest.AssertNotExistsBean(t, &issues_model.Issue{RepoID: repo.ID, Title: "banana title"})
	})
}

func TestAPIConfidentialIssueContract(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})
	writerToken := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteIssue)
	posterToken := getUserToken(t, "user10", auth_model.AccessTokenScopeWriteIssue)
	issuesURL := "/api/v1/repos/user2/repo1/issues"
	eventCount := func(t *testing.T, issueID int64, commentType issues_model.CommentType) int {
		return unittest.GetCount(t, &issues_model.Comment{IssueID: issueID, Type: commentType})
	}
	newIssue := func(t *testing.T, confidential bool) *issues_model.Issue {
		issue := &issues_model.Issue{RepoID: repo.ID, Repo: repo, Index: 9000 + int64(unittest.GetCount(t, &issues_model.Issue{RepoID: repo.ID})), PosterID: poster.ID, Poster: poster, Title: "api contract", IsConfidential: confidential}
		require.NoError(t, db.Insert(t.Context(), issue))
		return issue
	}
	issueURL := func(issue *issues_model.Issue) string {
		return fmt.Sprintf("%s/%d", issuesURL, issue.Index)
	}
	type errorBody struct {
		Message string `json:"message"`
	}

	t.Run("create", func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST", issuesURL, map[string]any{"title": "api secret", "confidential": true}).AddTokenAuth(writerToken)
		created := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Issue{})
		assert.True(t, created.Confidential)
		assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: created.ID}).IsConfidential)

		req = NewRequestWithJSON(t, "POST", issuesURL, map[string]any{"title": "api public"}).AddTokenAuth(writerToken)
		created = DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Issue{})
		assert.False(t, created.Confidential)

		req = NewRequestWithJSON(t, "POST", issuesURL, map[string]any{"title": "api banana", "confidential": "banana"}).AddTokenAuth(writerToken)
		body := DecodeJSON(t, MakeRequest(t, req, http.StatusUnprocessableEntity), &errorBody{})
		assert.Contains(t, body.Message, "confidential")
		unittest.AssertNotExistsBean(t, &issues_model.Issue{RepoID: repo.ID, Title: "api banana"})
	})

	t.Run("writer edits", func(t *testing.T) {
		issue := newIssue(t, false)
		for _, step := range []struct {
			value     bool
			eventType issues_model.CommentType
		}{{true, issues_model.CommentTypeMakeConfidential}, {false, issues_model.CommentTypeMakePublic}} {
			req := NewRequestWithJSON(t, "PATCH", issueURL(issue), map[string]any{"confidential": step.value}).AddTokenAuth(writerToken)
			assert.Equal(t, step.value, DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Issue{}).Confidential)
			assert.Equal(t, step.value, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID}).IsConfidential)
			assert.Equal(t, 1, eventCount(t, issue.ID, step.eventType))

			MakeRequest(t, NewRequestWithJSON(t, "PATCH", issueURL(issue), map[string]any{"confidential": step.value}).AddTokenAuth(writerToken), http.StatusCreated)
			assert.Equal(t, 1, eventCount(t, issue.ID, step.eventType), "same value writes no event")
		}
	})

	t.Run("absent value changes nothing", func(t *testing.T) {
		issue := newIssue(t, true)
		req := NewRequestWithJSON(t, "PATCH", issueURL(issue), map[string]any{"title": "renamed"}).AddTokenAuth(writerToken)
		assert.True(t, DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Issue{}).Confidential)
	})

	t.Run("poster without write access", func(t *testing.T) {
		issue := newIssue(t, false)
		req := NewRequestWithJSON(t, "PATCH", issueURL(issue), map[string]any{"title": "poster rename", "confidential": true}).AddTokenAuth(posterToken)
		body := DecodeJSON(t, MakeRequest(t, req, http.StatusForbidden), &errorBody{})
		assert.Equal(t, "You do not have permission to change the confidential setting of this issue.", body.Message)
		stored := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID})
		assert.False(t, stored.IsConfidential)
		assert.Equal(t, "api contract", stored.Title)

		req = NewRequestWithJSON(t, "PATCH", issueURL(issue), map[string]any{"confidential": false}).AddTokenAuth(posterToken)
		MakeRequest(t, req, http.StatusCreated)
		assert.Zero(t, eventCount(t, issue.ID, issues_model.CommentTypeMakePublic))
	})

	t.Run("pull request", func(t *testing.T) {
		pullURL := issuesURL + "/2"
		req := NewRequestWithJSON(t, "PATCH", pullURL, map[string]any{"confidential": true}).AddTokenAuth(writerToken)
		body := DecodeJSON(t, MakeRequest(t, req, http.StatusUnprocessableEntity), &errorBody{})
		assert.Equal(t, "Pull requests cannot be confidential.", body.Message)

		req = NewRequestWithJSON(t, "PATCH", pullURL, map[string]any{"confidential": false}).AddTokenAuth(writerToken)
		assert.False(t, DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Issue{}).Confidential)

		req = NewRequest(t, "GET", pullURL).AddTokenAuth(writerToken)
		assert.False(t, DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.Issue{}).Confidential)
	})

	t.Run("key always present", func(t *testing.T) {
		for _, link := range []string{issueURL(newIssue(t, false)), issueURL(newIssue(t, true)), issuesURL + "/2"} {
			req := NewRequest(t, "GET", link).AddTokenAuth(writerToken)
			raw := DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &map[string]any{})
			assert.IsType(t, true, (*raw)["confidential"], link)
		}
	})
}

var nonceValuePattern = regexp.MustCompile(`nonce(=\"|-)[0-9a-f]+`)

var renderTimingPattern = regexp.MustCompile(`(Page|Template): <strong>[0-9a-zµ.]+</strong>`)

var csrfValuePattern = regexp.MustCompile(`(?i)(_csrf|csrf_token)(["'=: ]+(?:value=)?["']?)[A-Za-z0-9_\-.:=]+`)

type confidentialWebFixture struct {
	issue          *issues_model.Issue
	commentID      int64
	issueUUID      string
	commentUUID    string
	missingIndex   int64
	makePublicByWr func(t *testing.T)
}

func newConfidentialWebFixture(t *testing.T, keyword string) *confidentialWebFixture {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.NoError(t, repo_service.AddOrUpdateCollaborator(t.Context(), repo, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}), perm.AccessModeWrite))
	poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})

	issue := &issues_model.Issue{RepoID: repo.ID, Repo: repo, Index: 9100, PosterID: poster.ID, Poster: poster, Title: "hidden " + keyword, Content: keyword, IsConfidential: true}
	require.NoError(t, db.Insert(t.Context(), issue))
	require.NoError(t, db.Insert(t.Context(), &issues_model.IssueAssignees{IssueID: issue.ID, AssigneeID: 8}))
	comment := &issues_model.Comment{IssueID: issue.ID, PosterID: poster.ID, Type: issues_model.CommentTypeComment, Content: keyword + " comment"}
	require.NoError(t, db.Insert(t.Context(), comment))

	uploader := loginUser(t, "user2")
	link := func(content string, comment *issues_model.Comment) string {
		uuid := testCreateIssueAttachment(t, uploader, "/user2/repo1", content+".txt", []byte(content), http.StatusOK)
		update := &repo_model.Attachment{IssueID: issue.ID}
		if comment != nil {
			update.CommentID = comment.ID
		}
		_, err := db.GetEngine(t.Context()).Where("uuid = ?", uuid).Update(update)
		require.NoError(t, err)
		return uuid
	}
	return &confidentialWebFixture{
		issue:        issue,
		commentID:    comment.ID,
		issueUUID:    link("on-issue", nil),
		commentUUID:  link("on-comment", comment),
		missingIndex: 99999,
		makePublicByWr: func(t *testing.T) {
			resp := loginUser(t, "user4").MakeRequest(t, NewRequestWithValues(t, "POST", fmt.Sprintf("/user2/repo1/issues/%d/confidential", issue.Index), map[string]string{"confidential": "false"}), http.StatusOK)
			require.NotEmpty(t, resp.Body.String())
		},
	}
}

// doNormalized sends the request and returns status and body with the issue index, CSP nonces, render timings and CSRF values masked.
func doNormalized(t *testing.T, session *TestSession, method, path string, index int64, values map[string]string) (int, string) {
	req := NewRequestWithValues(t, method, strings.ReplaceAll(path, "{index}", strconv.FormatInt(index, 10)), values).SetHeader("Accept", "text/html")
	resp := session.MakeRequest(t, req, NoExpectedStatus)
	body := nonceValuePattern.ReplaceAllString(resp.Body.String(), "nonce$1***")
	body = renderTimingPattern.ReplaceAllString(body, "$1: ***")
	body = strings.ReplaceAll(body, strconv.FormatInt(index, 10), "{index}")
	return resp.Code, csrfValuePattern.ReplaceAllString(body, "$1$2***")
}

func TestConfidentialIssueHiddenNotFound(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	fx := newConfidentialWebFixture(t, okapiKeyword)

	type route struct {
		method, path string
		values       map[string]string
	}
	routes := []route{
		{"GET", "/user2/repo1/issues/{index}", nil},
		{"GET", "/user2/repo1/pulls/{index}", nil},
		{"GET", "/user2/repo1/issues/{index}/info", nil},
		{"GET", "/user2/repo1/issues/{index}/attachments", nil},
		{"GET", "/user2/repo1/issues/{index}/content-history/overview", nil},
		{"POST", "/user2/repo1/issues/{index}/comments", map[string]string{"content": "hello"}},
		{"POST", "/user2/repo1/issues/{index}/title", map[string]string{"title": "changed"}},
		{"POST", "/user2/repo1/issues/{index}/content", map[string]string{"content": "changed"}},
		{"POST", "/user2/repo1/issues/{index}/watch", map[string]string{"watch": "1"}},
		{"POST", "/user2/repo1/issues/{index}/pin", nil},
		{"POST", "/user2/repo1/issues/{index}/lock", map[string]string{"reason": "off-topic"}},
		{"POST", "/user2/repo1/issues/{index}/reactions/react", map[string]string{"content": "+1"}},
		{"POST", "/user2/repo1/issues/{index}/dependency/add", map[string]string{"newDependency": "1"}},
		{"POST", "/user2/repo1/issues/{index}/dependency/delete", map[string]string{"removeDependencyID": "1", "dependencyType": "blockedBy"}},
		{"POST", "/user2/repo1/issues/{index}/delete", nil},
		{"POST", "/user2/repo1/issues/{index}/deadline", map[string]string{"deadline": "2030-01-01"}},
		{"POST", "/user2/repo1/issues/{index}/confidential", map[string]string{"confidential": "false"}},
	}
	batchRoutes := []route{
		{"POST", "/user2/repo1/issues/labels", map[string]string{"action": "attach", "id": "1"}},
		{"POST", "/user2/repo1/issues/assignee", map[string]string{"action": "set", "id": "2"}},
		{"POST", "/user2/repo1/issues/milestone", map[string]string{"id": "1"}},
		{"POST", "/user2/repo1/issues/projects", map[string]string{"id": "1"}},
	}

	assertState := func(t *testing.T) {
		current := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: fx.issue.ID})
		assert.True(t, current.IsConfidential)
		assert.Equal(t, "hidden "+okapiKeyword, current.Title)
		assert.Equal(t, okapiKeyword, current.Content)
		assert.Zero(t, current.MilestoneID)
		assert.False(t, current.IsLocked)
		unittest.AssertNotExistsBean(t, &issues_model.Comment{IssueID: fx.issue.ID, Content: "hello"})
	}

	for name, session := range map[string]*TestSession{"user5": loginUser(t, "user5"), "anonymous": emptyTestSession(t)} {
		t.Run("hidden "+name, func(t *testing.T) {
			for _, r := range routes {
				if name == "anonymous" && r.method == "POST" {
					continue // anonymous POST is answered by the sign-in redirect before any issue is loaded
				}
				wantStatus, wantBody := doNormalized(t, session, r.method, r.path, fx.missingIndex, r.values)
				gotStatus, gotBody := doNormalized(t, session, r.method, r.path, fx.issue.Index, r.values)
				assert.Equal(t, wantStatus, gotStatus, "%s %s", r.method, r.path)
				assert.Equal(t, wantBody, gotBody, "%s %s", r.method, r.path)
				if r.method == "GET" {
					assert.Equal(t, http.StatusNotFound, gotStatus, "%s %s", r.method, r.path)
				}
			}
			if name != "anonymous" {
				for _, r := range batchRoutes {
					values := map[string]string{"issue_ids": strconv.FormatInt(fx.issue.ID, 10)}
					maps.Copy(values, r.values)
					status, body := doNormalized(t, session, r.method, r.path, fx.issue.Index, values)
					assert.Equal(t, http.StatusNotFound, status, r.path)
					assert.NotContains(t, body, okapiKeyword)
				}
			}
			assertState(t)
		})
	}

	t.Run("allowed viewers", func(t *testing.T) {
		for _, user := range []string{"user4", "user10", "user8", "user2"} {
			session := loginUser(t, user)
			for _, path := range []string{"/user2/repo1/issues/{index}", "/user2/repo1/issues/{index}/info", "/user2/repo1/issues/{index}/attachments"} {
				status, _ := doNormalized(t, session, "GET", path, fx.issue.Index, nil)
				assert.Equal(t, http.StatusOK, status, "%s %s", user, path)
			}
		}
	})

	t.Run("unassigned assignee loses access", func(t *testing.T) {
		_, err := db.GetEngine(t.Context()).Where("issue_id = ? AND assignee_id = ?", fx.issue.ID, 8).Delete(&issues_model.IssueAssignees{})
		require.NoError(t, err)
		status, _ := doNormalized(t, loginUser(t, "user8"), "GET", "/user2/repo1/issues/{index}", fx.issue.Index, nil)
		assert.Equal(t, http.StatusNotFound, status)
	})

	t.Run("public after writer toggle", func(t *testing.T) {
		fx.makePublicByWr(t)
		for _, session := range []*TestSession{loginUser(t, "user5"), emptyTestSession(t)} {
			status, _ := doNormalized(t, session, "GET", "/user2/repo1/issues/{index}", fx.issue.Index, nil)
			assert.Equal(t, http.StatusOK, status)
		}
	})
}

func TestConfidentialIssueHiddenNotFoundAPI(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	fx := newConfidentialWebFixture(t, quokkaKeyword)
	issuesUnit := unittest.AssertExistsAndLoadBean(t, &repo_model.RepoUnit{RepoID: fx.issue.RepoID, Type: unit_model.TypeIssues})
	issuesUnit.Config = &repo_model.IssuesConfig{EnableTimetracker: true, EnableDependencies: true}
	_, err := db.GetEngine(t.Context()).ID(issuesUnit.ID).Cols("config").Update(issuesUnit)
	require.NoError(t, err)

	assertState := func(t *testing.T) {
		current := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: fx.issue.ID})
		assert.True(t, current.IsConfidential)
		assert.Equal(t, "hidden "+quokkaKeyword, current.Title)
		assert.Equal(t, quokkaKeyword+" comment", unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: fx.commentID}).Content)
	}

	const missingCommentID = 99999999
	type apiRoute struct {
		method, path string
		body         any
		needsAdmin   bool
		needsToken   bool
	}
	apiRoutes := []apiRoute{
		{"GET", "issues/{index}", nil, false, false},
		{"PATCH", "issues/{index}", map[string]any{"title": "changed"}, false, false},
		{"DELETE", "issues/{index}", nil, true, false},
		{"GET", "issues/{index}/comments", nil, false, false},
		{"POST", "issues/{index}/comments", map[string]any{"body": "hello"}, false, false},
		{"GET", "issues/comments/{comment}", nil, false, false},
		{"PATCH", "issues/comments/{comment}", map[string]any{"body": "changed"}, false, false},
		{"DELETE", "issues/comments/{comment}", nil, false, false},
		{"GET", "issues/{index}/timeline", nil, false, false},
		{"GET", "issues/{index}/reactions", nil, false, false},
		{"POST", "issues/{index}/reactions", map[string]any{"content": "+1"}, false, false},
		{"DELETE", "issues/{index}/reactions", map[string]any{"content": "+1"}, false, false},
		{"GET", "issues/comments/{comment}/reactions", nil, false, false},
		{"POST", "issues/comments/{comment}/reactions", map[string]any{"content": "+1"}, false, false},
		{"DELETE", "issues/comments/{comment}/reactions", map[string]any{"content": "+1"}, false, false},
		{"GET", "issues/{index}/assets", nil, false, false},
		{"GET", "issues/comments/{comment}/assets", nil, false, false},
		{"GET", "issues/{index}/dependencies", nil, false, false},
		{"POST", "issues/{index}/dependencies", map[string]any{"owner": "user2", "name": "repo1", "index": 1}, false, false},
		{"GET", "issues/{index}/blocks", nil, false, false},
		{"POST", "issues/{index}/blocks", map[string]any{"owner": "user2", "name": "repo1", "index": 1}, false, false},
		{"GET", "issues/{index}/labels", nil, false, false},
		{"POST", "issues/{index}/labels", map[string]any{"labels": []int64{1}}, false, false},
		{"GET", "issues/{index}/subscriptions", nil, false, false},
		{"GET", "issues/{index}/subscriptions/check", nil, false, true},
		{"POST", "issues/{index}/stopwatch/start", nil, false, false},
		{"POST", "issues/{index}/stopwatch/stop", nil, false, false},
		{"GET", "issues/{index}/times", nil, false, true},
		{"POST", "issues/{index}/pin", nil, true, false},
		{"POST", "issues/{index}/deadline", map[string]any{"due_date": "2030-01-01T00:00:00Z"}, false, false},
		{"POST", "issues/{index}/assignees", map[string]any{"assignees": []string{"user2"}}, false, false},
		{"GET", "issues/{index}/assignees/user2", nil, false, false},
	}
	idPattern := regexp.MustCompile(`id: \d+, `)
	indexPattern := regexp.MustCompile(`index: \d+\]`)
	call := func(t *testing.T, token string, r apiRoute, index, commentID int64) (int, string) {
		path := strings.NewReplacer("{index}", strconv.FormatInt(index, 10), "{comment}", strconv.FormatInt(commentID, 10)).Replace(r.path)
		req := NewRequestWithJSON(t, r.method, "/api/v1/repos/user2/repo1/"+path, r.body)
		if token != "" {
			req.AddTokenAuth(token)
		}
		resp := MakeRequest(t, req, NoExpectedStatus)
		return resp.Code, indexPattern.ReplaceAllString(idPattern.ReplaceAllString(resp.Body.String(), "id: X, "), "index: X]")
	}
	commentsURL := "/api/v1/repos/user2/repo1/issues/comments?limit=50"
	countComment := func(t *testing.T, token string) (comments []*api.Comment, total string) {
		req := NewRequest(t, "GET", commentsURL)
		if token != "" {
			req.AddTokenAuth(token)
		}
		resp := MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &comments)
		return comments, resp.Header().Get("X-Total-Count")
	}
	hasComment := func(comments []*api.Comment) bool {
		return slices.ContainsFunc(comments, func(c *api.Comment) bool { return c.ID == fx.commentID })
	}
	writerToken := getUserToken(t, "user4", auth_model.AccessTokenScopeWriteIssue)

	for name, token := range map[string]string{"user9": getUserToken(t, "user9", auth_model.AccessTokenScopeWriteIssue), "anonymous": ""} {
		t.Run("hidden "+name, func(t *testing.T) {
			for _, r := range apiRoutes {
				if r.needsAdmin || (token == "" && (r.method != "GET" || r.needsToken)) {
					continue // reqToken/reqAdmin answer before the issue is loaded
				}
				wantStatus, wantBody := call(t, token, r, fx.missingIndex, missingCommentID)
				gotStatus, gotBody := call(t, token, r, fx.issue.Index, fx.commentID)
				assert.Equal(t, http.StatusNotFound, gotStatus, "%s %s", r.method, r.path)
				assert.Equal(t, wantStatus, gotStatus, "%s %s", r.method, r.path)
				assert.Equal(t, wantBody, gotBody, "%s %s", r.method, r.path)
				assert.NotContains(t, gotBody, quokkaKeyword)
			}
			assertState(t)
			comments, total := countComment(t, token)
			assert.False(t, hasComment(comments))
			assert.Equal(t, strconv.Itoa(len(comments)), total)
		})
	}

	t.Run("site admin with public-only token", func(t *testing.T) {
		token := getUserToken(t, "user1", auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopePublicOnly)
		for _, r := range apiRoutes {
			wantStatus, wantBody := call(t, token, r, fx.missingIndex, missingCommentID)
			gotStatus, gotBody := call(t, token, r, fx.issue.Index, fx.commentID)
			assert.Equal(t, http.StatusNotFound, gotStatus, "%s %s", r.method, r.path)
			assert.Equal(t, wantStatus, gotStatus, "%s %s", r.method, r.path)
			assert.Equal(t, wantBody, gotBody, "%s %s", r.method, r.path)
		}
		assertState(t)
	})

	t.Run("allowed viewers", func(t *testing.T) {
		for _, user := range []string{"user4", "user10", "user8", "user2"} {
			token := getUserToken(t, user, auth_model.AccessTokenScopeWriteIssue)
			for _, r := range apiRoutes {
				if r.method != "GET" {
					continue
				}
				status, _ := call(t, token, r, fx.issue.Index, fx.commentID)
				assert.NotEqual(t, http.StatusNotFound, status, "%s %s %s", user, r.method, r.path)
			}
			comments, total := countComment(t, token)
			assert.True(t, hasComment(comments), user)
			assert.Equal(t, strconv.Itoa(len(comments)), total)
		}
	})

	t.Run("writer changes", func(t *testing.T) {
		status, _ := call(t, writerToken, apiRoute{method: "PATCH", path: "issues/comments/{comment}", body: map[string]any{"body": "writer edit"}}, fx.issue.Index, fx.commentID)
		assert.Equal(t, http.StatusOK, status)
	})

	t.Run("public after toggle", func(t *testing.T) {
		fx.makePublicByWr(t)
		for _, token := range []string{getUserToken(t, "user9", auth_model.AccessTokenScopeReadIssue), ""} {
			for _, r := range apiRoutes {
				if r.method != "GET" {
					continue
				}
				status, _ := call(t, token, r, fx.issue.Index, fx.commentID)
				assert.NotEqual(t, http.StatusNotFound, status, "%s %s", r.method, r.path)
			}
			comments, _ := countComment(t, token)
			assert.True(t, hasComment(comments))
		}
	})
}

func TestConfidentialIssueAttachments(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	fx := newConfidentialWebFixture(t, numbatKeyword)

	const unknownUUID = "00000000-0000-0000-0000-000000000000"
	attachmentLinks := func(uuid string) []string {
		return []string{"/attachments/" + uuid, "/user2/repo1/attachments/" + uuid}
	}

	for _, uuid := range []string{fx.issueUUID, fx.commentUUID} {
		for i, link := range attachmentLinks(uuid) {
			unknownLink := attachmentLinks(unknownUUID)[i]
			for name, session := range map[string]*TestSession{"user5": loginUser(t, "user5"), "anonymous": emptyTestSession(t)} {
				wantStatus, wantBody := doNormalized(t, session, "GET", unknownLink, 0, nil)
				require.Equal(t, http.StatusNotFound, wantStatus)
				status, body := doNormalized(t, session, "GET", link, 0, nil)
				assert.Equal(t, wantStatus, status, "%s %s", name, link)
				assert.Equal(t, wantBody, strings.ReplaceAll(body, uuid, unknownUUID), "%s %s", name, link)
			}
			for _, user := range []string{"user4", "user10", "user8", "user2"} {
				status, _ := doNormalized(t, loginUser(t, user), "GET", link, 0, nil)
				assert.Equal(t, http.StatusOK, status, "%s %s", user, link)
			}
		}
	}

	fx.makePublicByWr(t)
	for _, uuid := range []string{fx.issueUUID, fx.commentUUID} {
		for _, session := range []*TestSession{loginUser(t, "user5"), emptyTestSession(t)} {
			status, _ := doNormalized(t, session, "GET", "/attachments/"+uuid, 0, nil)
			assert.Equal(t, http.StatusOK, status)
		}
	}
}

const (
	okapiKeyword   = "okapibunting"
	quokkaKeyword  = "quokkawren"
	numbatKeyword  = "numbatlark"
	zebraKeyword   = "zebrafinch"
	gerenukKeyword = "gerenukpipit"
	tapirKeyword   = "tapirshrike"
)

// newConfidentialListIssue inserts "hidden <keyword>" in user2/repo1: posted by user10, assigned to user8.
func newConfidentialListIssue(t *testing.T, keyword string, confidential bool) *issues_model.Issue {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})
	issue := &issues_model.Issue{RepoID: repo.ID, Repo: repo, Index: 9100, PosterID: poster.ID, Poster: poster, Title: "hidden " + keyword, Content: keyword, IsConfidential: confidential}
	require.NoError(t, db.Insert(t.Context(), issue))
	require.NoError(t, db.Insert(t.Context(), &issues_model.IssueAssignees{IssueID: issue.ID, AssigneeID: 8}))
	return issue
}

func setConfidentialColumn(t *testing.T, issue *issues_model.Issue, confidential bool) {
	_, err := db.GetEngine(t.Context()).ID(issue.ID).Cols("is_confidential").Update(&issues_model.Issue{IsConfidential: confidential})
	require.NoError(t, err)
}

// confidentialViewer is a user name, "" is anonymous.
type confidentialViewer string

func (v confidentialViewer) request(t *testing.T, link string) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", link)
	if v == "" {
		return MakeRequest(t, req, http.StatusOK)
	}
	return loginUser(t, string(v)).MakeRequest(t, req, http.StatusOK)
}

func (v confidentialViewer) apiRequest(t *testing.T, link string) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", link)
	if v != "" {
		req.AddTokenAuth(getUserToken(t, string(v), auth_model.AccessTokenScopeReadIssue, auth_model.AccessTokenScopeReadRepository))
	}
	return MakeRequest(t, req, http.StatusOK)
}

// webIssueRows returns the row title and whether it carries the Confidential marker, per row of the list.
func webIssueRows(t *testing.T, resp *httptest.ResponseRecorder) (titles []string, markers map[string]bool) {
	markers = map[string]bool{}
	NewHTMLParser(t, resp.Body).doc.Find("#issue-list > .item").Each(func(_ int, item *goquery.Selection) {
		title := strings.TrimSpace(item.Find(".list-item-large-title").Text())
		titles = append(titles, title)
		markers[title] = strings.Contains(item.Find(".ui.warning.label").Text(), "Confidential")
	})
	return titles, markers
}

func apiIssueTitles(t *testing.T, resp *httptest.ResponseRecorder) (titles []string, total string) {
	for _, issue := range *DecodeJSON(t, resp, &[]api.Issue{}) {
		titles = append(titles, issue.Title)
	}
	return titles, resp.Header().Get("X-Total-Count")
}

func TestConfidentialIssueLists(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issue := newConfidentialListIssue(t, zebraKeyword, true)
	require.NoError(t, db.Insert(t.Context(), &issues_model.IssuePin{RepoID: issue.RepoID, IssueID: issue.ID, PinOrder: 1}))
	issue_indexer.UpdateIssueIndexer(t.Context(), issue.ID)
	require.Eventually(t, func() bool {
		ids, _, err := issue_indexer.SearchIssues(t.Context(), &issue_indexer.SearchOptions{Keyword: zebraKeyword, RepoIDs: []int64{issue.RepoID}, Visibility: issue_indexer.ToVisibilityFilter(&issues_model.VisibilityScope{Unrestricted: true})})
		return err == nil && slices.Contains(ids, issue.ID)
	}, 10*time.Second, 10*time.Millisecond)

	webLists := []string{
		"/user2/repo1/issues?state=all",
		"/user2/repo1/issues?state=open",
		"/user2/repo1/issues?state=all&sort=oldest",
		"/user2/repo1/issues?state=all&poster=user10",
		"/user2/repo1/issues?state=all&assignee=8",
	}
	apiLists := []string{
		"/api/v1/repos/user2/repo1/issues?state=all&type=issues&limit=50",
		"/api/v1/repos/issues/search?state=all&limit=50",
		"/api/v1/repos/user2/repo1/issues/pinned",
	}
	dashboard := map[confidentialViewer]string{"user10": "/issues?type=created_by&state=open", "user8": "/issues?type=assigned&state=open", "user2": "/issues?type=your_repositories&state=open"}

	check := func(t *testing.T, canSee map[confidentialViewer]bool, marked bool) {
		for viewer, sees := range canSee {
			for _, link := range webLists {
				titles, markers := webIssueRows(t, viewer.request(t, link))
				assert.Equal(t, sees, slices.Contains(titles, issue.Title), "%q %s", viewer, link)
				assert.Equal(t, sees && marked, markers[issue.Title], "marker %q %s", viewer, link)
			}
			if link, ok := dashboard[viewer]; ok {
				titles, markers := webIssueRows(t, viewer.request(t, link))
				assert.Equal(t, sees, slices.Contains(titles, issue.Title), "dashboard %q", viewer)
				assert.Equal(t, sees && marked, markers[issue.Title], "dashboard marker %q", viewer)
			}
			titles, _ := webIssueRows(t, viewer.request(t, "/user2/repo1/pulls?state=all"))
			assert.NotContains(t, titles, issue.Title, "pulls %q", viewer)
			assert.Equal(t, sees, strings.Contains(viewer.request(t, "/user2/repo1/issues").Body.String(), issue.Title), "pinned %q", viewer)

			for _, link := range apiLists {
				titles, total := apiIssueTitles(t, viewer.apiRequest(t, link))
				assert.Equal(t, sees, slices.Contains(titles, issue.Title), "%q %s", viewer, link)
				if !strings.HasSuffix(link, "/pinned") {
					assert.Equal(t, strconv.Itoa(len(titles)), total, "%q %s", viewer, link)
				}
			}
		}
	}

	t.Run("confidential", func(t *testing.T) {
		check(t, map[confidentialViewer]bool{"user2": true, "user10": true, "user8": true, "user4": false, "": false}, true)

		hiddenTotal := func(v confidentialViewer) string {
			_, total := apiIssueTitles(t, v.apiRequest(t, apiLists[0]))
			return total
		}
		writerTotal, err := strconv.Atoi(hiddenTotal("user2"))
		require.NoError(t, err)
		assert.Equal(t, strconv.Itoa(writerTotal-1), hiddenTotal("user4"))
	})

	t.Run("keyword", func(t *testing.T) {
		titles, _ := webIssueRows(t, confidentialViewer("user2").request(t, "/user2/repo1/issues?q="+zebraKeyword))
		assert.Equal(t, []string{issue.Title}, titles)
		for _, viewer := range []confidentialViewer{"user4", ""} {
			titles, _ = webIssueRows(t, viewer.request(t, "/user2/repo1/issues?q="+zebraKeyword))
			assert.Empty(t, titles, "%q", viewer)
			apiTitles, total := apiIssueTitles(t, viewer.apiRequest(t, "/api/v1/repos/issues/search?state=all&q="+zebraKeyword))
			assert.Empty(t, apiTitles, "%q", viewer)
			assert.Equal(t, "0", total, "%q", viewer)
		}
	})

	t.Run("confidential filter", func(t *testing.T) {
		control := func(htmlDoc *HTMLDoc) int {
			return htmlDoc.Find("div.item.ui.dropdown.jump > span.text").FilterFunction(func(_ int, text *goquery.Selection) bool {
				return strings.TrimSpace(text.Text()) == "Visibility"
			}).Length()
		}
		writer := confidentialViewer("user2")
		resp := writer.request(t, "/user2/repo1/issues?state=all&confidential=true")
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Equal(t, 1, control(htmlDoc))
		assert.Equal(t, 1, htmlDoc.Find("#issue-list > .item").Length())
		assert.Contains(t, htmlDoc.Find("#issue-list > .item").Text(), issue.Title)

		titles, _ := webIssueRows(t, writer.request(t, "/user2/repo1/issues?state=all&confidential=false"))
		assert.NotEmpty(t, titles)
		assert.NotContains(t, titles, issue.Title)

		for _, viewer := range []confidentialViewer{"user4", ""} {
			htmlDoc = NewHTMLParser(t, viewer.request(t, "/user2/repo1/issues?state=all&confidential=true").Body)
			assert.Equal(t, 0, htmlDoc.Find("#issue-list > .item").Length(), "%q", viewer)
			assert.Equal(t, 0, control(htmlDoc), "%q", viewer)
		}
		resp = confidentialViewer("user10").request(t, "/user2/repo1/issues?state=all")
		assert.Equal(t, 0, control(NewHTMLParser(t, resp.Body)), "poster is not a Writer")
	})

	t.Run("published", func(t *testing.T) {
		setConfidentialColumn(t, issue, false)
		check(t, map[confidentialViewer]bool{"user2": true, "user10": true, "user8": true, "user4": true, "": true}, false)
	})
}

func TestConfidentialIssueSearchStaleIndex(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issue := newConfidentialListIssue(t, gerenukKeyword, false)
	issue_indexer.UpdateIssueIndexer(t.Context(), issue.ID)
	require.Eventually(t, func() bool {
		ids, _, err := issue_indexer.SearchIssues(t.Context(), &issue_indexer.SearchOptions{Keyword: gerenukKeyword, RepoIDs: []int64{issue.RepoID}})
		return err == nil && slices.Contains(ids, issue.ID)
	}, 10*time.Second, 10*time.Millisecond)

	setConfidentialColumn(t, issue, true) // no indexer update: the index still holds a public document

	for _, viewer := range []confidentialViewer{"user4", ""} {
		titles, _ := webIssueRows(t, viewer.request(t, "/user2/repo1/issues?q="+gerenukKeyword))
		assert.Empty(t, titles, "web %q", viewer)
		apiTitles, _ := apiIssueTitles(t, viewer.apiRequest(t, "/api/v1/repos/issues/search?state=all&q="+gerenukKeyword))
		assert.Empty(t, apiTitles, "api search %q", viewer)
		apiTitles, _ = apiIssueTitles(t, viewer.apiRequest(t, "/api/v1/repos/user2/repo1/issues?state=all&q="+gerenukKeyword))
		assert.Empty(t, apiTitles, "api list %q", viewer)
	}
}

type statementCounter struct {
	active atomic.Bool
	count  atomic.Int64
}

func (c *statementCounter) BeforeProcess(h *contexts.ContextHook) (context.Context, error) {
	if c.active.Load() {
		c.count.Add(1)
	}
	return h.Ctx, nil
}

func (*statementCounter) AfterProcess(*contexts.ContextHook) error { return nil }

func TestConfidentialIssueQueryCount(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	counter := &statementCounter{}
	unittest.GetXORMEngine().AddHook(counter)

	// the minimum over runs excludes statements from background work
	statements := func(viewer string) int64 {
		session := loginUser(t, viewer)
		req := NewRequest(t, "GET", "/user2/repo1/issues?state=all&poster=user10")
		session.MakeRequest(t, req, http.StatusOK) // warm caches
		fewest := int64(math.MaxInt64)
		for range 5 {
			counter.count.Store(0)
			counter.active.Store(true)
			session.MakeRequest(t, req, http.StatusOK)
			counter.active.Store(false)
			fewest = min(fewest, counter.count.Load())
		}
		return fewest
	}
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})
	addIssues := func(from, to int) {
		for i := from; i < to; i++ {
			require.NoError(t, db.Insert(t.Context(), &issues_model.Issue{RepoID: repo.ID, Repo: repo, Index: int64(9300 + i), PosterID: poster.ID, Poster: poster, Title: "bulk"}))
		}
	}

	// a page that is not full skips the total-count statement of the list, so both sizes fill the page
	addIssues(0, setting.UI.IssuePagingNum+1)
	hiddenOne, writerOne := statements("user4"), statements("user2")
	// one for the visible-issue filter of the list and one for the Issues badge (CountOpenIssuesForViewer)
	assert.LessOrEqual(t, hiddenOne, writerOne+2, "no confidential issues: at most one more statement per feature for a hidden viewer")

	addIssues(setting.UI.IssuePagingNum+1, 50)
	assert.Equal(t, hiddenOne, statements("user4"), "hidden viewer: statements do not grow with the number of issues")
	assert.Equal(t, writerOne, statements("user2"), "writer: statements do not grow with the number of issues")
}

func TestConfidentialIssueCounts(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("IssuesBadge", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		expected := strconv.Itoa(repo.NumIssues - repo.NumClosedIssues)
		require.NotEqual(t, "0", expected)

		issuesTabBadge := func(htmlDoc *HTMLDoc) string {
			return strings.TrimSpace(htmlDoc.Find(`a.item[href="/user2/repo1/issues"] .ui.label`).Text())
		}

		resp := MakeRequest(t, NewRequest(t, "GET", "/user2/repo1"), http.StatusOK)
		assert.Equal(t, expected, issuesTabBadge(NewHTMLParser(t, resp.Body)), "anonymous")

		resp = loginUser(t, "user2").MakeRequest(t, NewRequest(t, "GET", "/user2/repo1"), http.StatusOK)
		assert.Equal(t, expected, issuesTabBadge(NewHTMLParser(t, resp.Body)), "owner")
	})

	t.Run("MilestoneCounts", func(t *testing.T) {
		resp := MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/milestones"), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		type milestoneRow struct{ open, closed, percent string }
		rows := map[string]milestoneRow{}
		htmlDoc.Find(".milestone-list > .item").Each(func(_ int, item *goquery.Selection) {
			counts := item.Find(".list-item-secondary-bar .flex-text-inline")
			rows[strings.TrimSpace(item.Find(".list-item-large-title a").Text())] = milestoneRow{
				open:    strings.Fields(counts.Eq(0).Text())[0],
				closed:  strings.Fields(counts.Eq(1).Text())[0],
				percent: strings.TrimSpace(item.Find(".flex-left-right .flex-text-inline span").First().Text()),
			}
		})

		assert.Equal(t, map[string]milestoneRow{
			"milestone1": {open: "1", closed: "0", percent: "0%"},
			"milestone2": {open: "0", closed: "0", percent: "0%"},
		}, rows)
	})

	t.Run("ProjectColumns", func(t *testing.T) {
		// project 1: issues 1 and 2 (no column assigned) in the default "To Do", issue 3 "In Progress", issue 5 "Done"
		resp := MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/projects/1"), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		counts := map[string]string{}
		htmlDoc.Find(".project-column").Each(func(_ int, column *goquery.Selection) {
			counts[strings.TrimSpace(column.Find(".project-column-title-text").Text())] = strings.TrimSpace(column.Find(".project-column-issue-count").Text())
		})
		assert.Equal(t, map[string]string{"To Do": "2", "In Progress": "1", "Done": "1"}, counts)
	})

	// issue 1 (open, label 1, project 1 "To Do") becomes confidential; issues 1 and 5 (closed) join milestone 1: 3 issues, 1 closed
	ctx := t.Context()
	poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})
	for i := range 2 {
		require.NoError(t, db.Insert(ctx, &issues_model.Issue{RepoID: 1, Index: int64(9400 + i), PosterID: poster.ID, Title: "public open"}))
	}
	_, err := db.GetEngine(ctx).Exec("UPDATE repository SET num_issues = num_issues + 2 WHERE id = 1")
	require.NoError(t, err)
	repoBefore := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	for _, issueID := range []int64{1, 5} {
		_, err := db.GetEngine(ctx).ID(issueID).Cols("milestone_id").Update(&issues_model.Issue{MilestoneID: 1})
		require.NoError(t, err)
	}
	_, err = db.GetEngine(ctx).ID(1).Cols("num_issues", "num_closed_issues", "completeness").Update(&issues_model.Milestone{NumIssues: 3, NumClosedIssues: 1, Completeness: 33})
	require.NoError(t, err)
	_, err = db.GetEngine(ctx).ID(1).Cols("created_unix").Update(&issues_model.Issue{CreatedUnix: timeutil.TimeStampNow()})
	require.NoError(t, err)
	setConfidentialColumn(t, &issues_model.Issue{ID: 1}, true)

	openIssues := repoBefore.NumIssues - repoBefore.NumClosedIssues
	const hidden, writer confidentialViewer = "user4", "user2"

	milestoneRow := func(viewer confidentialViewer) (open, closed, percent string) {
		item := NewHTMLParser(t, viewer.request(t, "/user2/repo1/milestones").Body).Find(".milestone-list > .item").First()
		counts := item.Find(".list-item-secondary-bar .flex-text-inline")
		return strings.Fields(counts.Eq(0).Text())[0], strings.Fields(counts.Eq(1).Text())[0], strings.TrimSpace(item.Find(".flex-left-right .flex-text-inline span").First().Text())
	}
	columnCounts := func(viewer confidentialViewer) map[string]string {
		counts := map[string]string{}
		NewHTMLParser(t, viewer.request(t, "/user2/repo1/projects/1").Body).Find(".project-column").Each(func(_ int, column *goquery.Selection) {
			counts[strings.TrimSpace(column.Find(".project-column-title-text").Text())] = strings.TrimSpace(column.Find(".project-column-issue-count").Text())
		})
		return counts
	}
	labelOpenCount := func(viewer confidentialViewer) string {
		return strings.Fields(NewHTMLParser(t, viewer.request(t, "/user2/repo1/labels").Body).Find(`a.open-issues[href="/user2/repo1/issues?labels=1"]`).Text())[0]
	}
	badge := func(viewer confidentialViewer) string {
		return strings.TrimSpace(NewHTMLParser(t, viewer.request(t, "/user2/repo1").Body).Find(`a.item[href="/user2/repo1/issues"] .ui.label`).Text())
	}
	activityHasIssue := func(viewer confidentialViewer) bool {
		return strings.Contains(viewer.request(t, "/user2/repo1/activity/monthly").Body.String(), `/user2/repo1/issues/1"`)
	}

	assert.Equal(t, strconv.Itoa(openIssues-1), badge(hidden))
	assert.Equal(t, strconv.Itoa(openIssues), badge(writer))

	open, closed, percent := milestoneRow(hidden)
	assert.Equal(t, []string{"1", "1", "50%"}, []string{open, closed, percent}, "hidden milestone list")
	open, closed, percent = milestoneRow(writer)
	assert.Equal(t, []string{"2", "1", "33%"}, []string{open, closed, percent}, "writer milestone list")

	for viewer, expected := range map[confidentialViewer][2]int{hidden: {1, 1}, writer: {2, 1}} {
		milestone := DecodeJSON(t, viewer.apiRequest(t, "/api/v1/repos/user2/repo1/milestones/1"), &api.Milestone{})
		assert.Equal(t, expected, [2]int{milestone.OpenIssues, milestone.ClosedIssues}, "api milestone %q", viewer)
		list := *DecodeJSON(t, viewer.apiRequest(t, "/api/v1/repos/user2/repo1/milestones?state=all"), &[]api.Milestone{})
		require.NotEmpty(t, list)
		assert.Equal(t, expected, [2]int{list[0].OpenIssues, list[0].ClosedIssues}, "api milestone list %q", viewer)
	}

	assert.Equal(t, "1", labelOpenCount(hidden))
	assert.Equal(t, "2", labelOpenCount(writer))

	assert.Equal(t, "1", columnCounts(hidden)["To Do"])
	assert.Equal(t, "2", columnCounts(writer)["To Do"])

	assert.False(t, activityHasIssue(hidden))
	assert.True(t, activityHasIssue(writer))

	repoAfter := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Equal(t, repoBefore.NumIssues, repoAfter.NumIssues)
	assert.Equal(t, repoBefore.NumClosedIssues, repoAfter.NumClosedIssues)
	milestone := unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1})
	assert.Equal(t, 3, milestone.NumIssues)
	assert.Equal(t, 1, milestone.NumClosedIssues)
}

const confidentialFeedTitle = "kestrelsecret"

func TestConfidentialIssueFeedsAndNotifications(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("RepoFeed", func(t *testing.T) {
		session := loginUser(t, "user2")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteRepository)

		const issueTitle = "feed issue"
		req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/issues", &api.CreateIssueOption{Title: issueTitle}).AddTokenAuth(token)
		issue := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Issue{})

		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/user2/repo1/issues/%d/comments", issue.Index), &api.CreateIssueCommentOption{Body: "feed comment"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		closed := string(api.StateClosed)
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/user2/repo1/issues/%d", issue.Index), &api.EditIssueOption{State: &closed}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		resp := MakeRequest(t, NewRequest(t, "GET", "/user2/repo1.rss"), http.StatusOK)
		var rss RSS
		require.NoError(t, xml.Unmarshal(resp.Body.Bytes(), &rss))

		var titles []string
		for _, item := range rss.Channel.Items {
			if strings.Contains(item.Title, issueTitle) || strings.Contains(item.Description, issueTitle) || strings.Contains(item.Link, fmt.Sprintf("/issues/%d", issue.Index)) {
				titles = append(titles, item.Title)
			}
		}
		assert.Len(t, titles, 3)
	})
	ctx := t.Context()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := &issues_model.Issue{RepoID: repo.ID, Repo: repo, Index: 9300, PosterID: 2, Title: confidentialFeedTitle, IsConfidential: false}
	require.NoError(t, db.Insert(ctx, issue))
	// rows are written while the issue is still public, and the flag is toggled afterwards
	for _, userID := range []int64{2, 5} {
		require.NoError(t, db.Insert(ctx, &activities_model.Action{
			UserID: userID, ActUserID: 2, RepoID: repo.ID, OpType: activities_model.ActionCreateIssue, Content: fmt.Sprintf("%d|%s", issue.Index, issue.Title),
		}))
		require.NoError(t, db.Insert(ctx, &activities_model.Notification{
			UserID: userID, RepoID: repo.ID, IssueID: issue.ID, Status: activities_model.NotificationStatusUnread, Source: activities_model.NotificationSourceIssue, UpdatedBy: 2,
		}))
	}

	pages := []string{"/", "/user2?tab=activity", "/user2.rss", "/user2.atom", "/user2/repo1.rss", "/user2/repo1.atom", "/notifications?type=unread"}
	mentions := func(viewer confidentialViewer, link string) bool {
		return strings.Contains(viewer.request(t, link).Body.String(), confidentialFeedTitle)
	}
	apiNew := func(viewer string) int64 {
		token := getUserToken(t, viewer, auth_model.AccessTokenScopeReadNotification)
		req := NewRequest(t, "GET", "/api/v1/notifications/new").AddTokenAuth(token)
		return DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.NotificationCount{}).New
	}
	apiListed := func(viewer string) int {
		token := getUserToken(t, viewer, auth_model.AccessTokenScopeReadNotification)
		for _, link := range []string{"/api/v1/notifications?all=true", "/api/v1/repos/user2/repo1/notifications?all=true"} {
			req := NewRequest(t, "GET", link).AddTokenAuth(token)
			for _, thread := range *DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &[]api.NotificationThread{}) {
				if thread.Subject != nil && thread.Subject.Title == confidentialFeedTitle {
					return 1
				}
			}
		}
		return 0
	}

	newBefore := map[string]int64{"user2": apiNew("user2"), "user5": apiNew("user5")}
	assert.True(t, mentions("user5", "/"), "public dashboard")

	setConfidentialColumn(t, issue, true)
	for _, link := range pages {
		assert.True(t, mentions("user2", link), "writer %s", link)
		assert.False(t, mentions("user5", link), "hidden viewer %s", link)
	}
	assert.Equal(t, 1, apiListed("user2"))
	assert.Equal(t, 0, apiListed("user5"))
	assert.Equal(t, newBefore["user2"], apiNew("user2"))
	assert.Equal(t, newBefore["user5"]-1, apiNew("user5"))

	setConfidentialColumn(t, issue, false)
	for _, link := range []string{"/", "/user2?tab=activity", "/user2/repo1.rss", "/notifications?type=unread"} {
		assert.True(t, mentions("user5", link), "published %s", link)
	}
	assert.Equal(t, 1, apiListed("user5"))
	assert.Equal(t, newBefore["user5"], apiNew("user5"))
}

func TestAPIConfidentialRepoOpenIssuesCount(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	var confidentialIssues []*issues_model.Issue
	for index := range int64(4) {
		issue := &issues_model.Issue{RepoID: repo.ID, Repo: repo, Index: 9000 + index, PosterID: owner.ID, Poster: owner, Title: "count", IsConfidential: index >= 2}
		require.NoError(t, db.Insert(t.Context(), issue))
		if issue.IsConfidential {
			confidentialIssues = append(confidentialIssues, issue)
		}
	}
	repo.NumIssues = repo.NumClosedIssues + 5 // 1 fixture + 2 public + 2 confidential open issues
	_, err := db.GetEngine(t.Context()).ID(repo.ID).Cols("num_issues").Update(repo)
	require.NoError(t, err)

	writerToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopeReadUser)
	hiddenToken := getUserToken(t, "user9", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopeReadUser)
	findRepo := func(repos []*api.Repository) *api.Repository {
		for _, r := range repos {
			if r.ID == repo.ID {
				return r
			}
		}
		require.FailNow(t, "repository missing from the list")
		return nil
	}
	openIssuesCounts := func(token string) map[string]int {
		get := func(url string) *RequestWrapper {
			req := NewRequest(t, "GET", url)
			if token != "" {
				req.AddTokenAuth(token)
			}
			return req
		}
		counts := map[string]int{
			"repo":   DecodeJSON(t, MakeRequest(t, get("/api/v1/repos/user2/repo1"), http.StatusOK), &api.Repository{}).OpenIssues,
			"search": findRepo(DecodeJSON(t, MakeRequest(t, get("/api/v1/repos/search?q=repo1"), http.StatusOK), &api.SearchResults{}).Data).OpenIssues,
			"list":   findRepo(*DecodeJSON(t, MakeRequest(t, get("/api/v1/users/user2/repos"), http.StatusOK), &[]*api.Repository{})).OpenIssues,
		}
		if token != "" { // GET /repositories/{id} requires a token
			counts["id"] = DecodeJSON(t, MakeRequest(t, get("/api/v1/repositories/1"), http.StatusOK), &api.Repository{}).OpenIssues
		}
		return counts
	}
	assertAll := func(expected int, token string) {
		for name, count := range openIssuesCounts(token) {
			assert.Equal(t, expected, count, name)
		}
	}

	assertAll(3, "")
	assertAll(3, hiddenToken)
	assertAll(5, writerToken)

	confidentialIssues[0].IsConfidential = false
	require.NoError(t, issues_model.UpdateIssueCols(t.Context(), confidentialIssues[0], "is_confidential"))
	assertAll(4, hiddenToken)
}

func TestConfidentialIssueIncomingMail(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	setConfidentialColumn(t, issue, true)
	issue.IsConfidential = true
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	former := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	// the reply and unsubscribe tokens were issued while the user was assigned
	removed, _, err := issues_model.ToggleIssueAssignee(t.Context(), issue, owner, former.ID)
	require.NoError(t, err)
	require.False(t, removed)
	payload, err := incoming_payload.CreateReferencePayload(issue)
	require.NoError(t, err)
	require.NoError(t, issues_model.CreateOrUpdateIssueWatch(t.Context(), former.ID, issue.ID, true))
	removed, _, err = issues_model.ToggleIssueAssignee(t.Context(), issue, owner, former.ID)
	require.NoError(t, err)
	require.True(t, removed)

	commentCount := unittest.GetCount(t, &issues_model.Comment{IssueID: issue.ID})
	content := &incoming.MailContent{Content: "reply from a former assignee"}
	require.NoError(t, (&incoming.ReplyHandler{}).Handle(t.Context(), content, former, payload))
	assert.Equal(t, commentCount, unittest.GetCount(t, &issues_model.Comment{IssueID: issue.ID}))

	require.NoError(t, (&incoming.UnsubscribeHandler{}).Handle(t.Context(), content, former, payload))
	watching, err := issues_model.CheckIssueWatch(t.Context(), former, issue)
	require.NoError(t, err)
	assert.True(t, watching)

	// control: the same reply from the poster is accepted
	poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: issue.PosterID})
	require.NoError(t, (&incoming.ReplyHandler{}).Handle(t.Context(), content, poster, payload))
	assert.Equal(t, commentCount+1, unittest.GetCount(t, &issues_model.Comment{IssueID: issue.ID}))
}

func TestConfidentialIssueReferencesAndDependencies(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	enableRepoDependencies(t, repo.ID)
	publicIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID, Index: 1})
	confidential := newConfidentialListIssue(t, tapirKeyword, true)
	// the reference comment that "see #P" in the body of the confidential issue creates on the public issue
	require.NoError(t, db.Insert(t.Context(), &issues_model.Comment{
		Type: issues_model.CommentTypeIssueRef, IssueID: publicIssue.ID, PosterID: confidential.PosterID,
		RefRepoID: repo.ID, RefIssueID: confidential.ID, RefAction: 0,
	}))
	require.NoError(t, db.Insert(t.Context(), &issues_model.IssueDependency{UserID: 2, IssueID: publicIssue.ID, DependencyID: confidential.ID}))
	require.NoError(t, db.Insert(t.Context(), &issues_model.IssueDependency{UserID: 2, IssueID: confidential.ID, DependencyID: publicIssue.ID}))
	dependencyCommentTypes := []string{"add_dependency", "remove_dependency"}
	for _, commentType := range []issues_model.CommentType{issues_model.CommentTypeAddDependency, issues_model.CommentTypeRemoveDependency} {
		require.NoError(t, db.Insert(t.Context(), &issues_model.Comment{
			Type: commentType, IssueID: publicIssue.ID, PosterID: 2, DependentIssueID: confidential.ID,
		}))
	}

	issueLink := fmt.Sprintf("/%s/issues/%d", repo.FullName(), publicIssue.Index)
	apiLink := fmt.Sprintf("/api/v1/repos/%s/issues/%d", repo.FullName(), publicIssue.Index)
	confidentialLink := fmt.Sprintf("/%s/issues/%d", repo.FullName(), confidential.Index)

	checkHidden := func(t *testing.T, viewer confidentialViewer) {
		page := viewer.request(t, issueLink).Body.String()
		assert.Contains(t, page, "referenced a confidential issue")
		assert.Contains(t, page, "You do not have permission to read 1 dependency")
		assert.NotContains(t, page, tapirKeyword)
		assert.NotContains(t, page, confidentialLink)

		timeline := viewer.apiRequest(t, apiLink+"/timeline")
		var refComments []*api.TimelineComment
		for _, c := range *DecodeJSON(t, timeline, &[]*api.TimelineComment{}) {
			if c.Type == "issue_ref" {
				refComments = append(refComments, c)
			}
		}
		require.Len(t, refComments, 1)
		assert.Equal(t, "referenced a confidential issue", refComments[0].Body)
		assert.Nil(t, refComments[0].RefIssue)
		assert.Nil(t, refComments[0].RefComment)
		assert.Empty(t, refComments[0].RefCommitSHA)
		assert.Empty(t, refComments[0].RefAction)
		assert.NotContains(t, timeline.Body.String(), tapirKeyword)
		for _, c := range *DecodeJSON(t, viewer.apiRequest(t, apiLink+"/timeline"), &[]*api.TimelineComment{}) {
			assert.NotContains(t, dependencyCommentTypes, c.Type)
		}

		for _, list := range []string{"/dependencies", "/blocks"} {
			titles, total := apiIssueTitles(t, viewer.apiRequest(t, apiLink+list))
			assert.Empty(t, titles, list)
			assert.Equal(t, "0", total, list)
		}
	}
	checkVisible := func(t *testing.T, viewer confidentialViewer) {
		page := viewer.request(t, issueLink).Body.String()
		assert.NotContains(t, page, "referenced a confidential issue")
		assert.Contains(t, page, tapirKeyword)
		assert.Contains(t, page, confidentialLink)

		var refComment *api.TimelineComment
		for _, c := range *DecodeJSON(t, viewer.apiRequest(t, apiLink+"/timeline"), &[]*api.TimelineComment{}) {
			if c.Type == "issue_ref" {
				refComment = c
			}
		}
		require.NotNil(t, refComment)
		require.NotNil(t, refComment.RefIssue)
		assert.Equal(t, confidential.Title, refComment.RefIssue.Title)

		var dependencyTitles []string
		for _, c := range *DecodeJSON(t, viewer.apiRequest(t, apiLink+"/timeline"), &[]*api.TimelineComment{}) {
			if slices.Contains(dependencyCommentTypes, c.Type) {
				require.NotNil(t, c.DependentIssue)
				dependencyTitles = append(dependencyTitles, c.DependentIssue.Title)
			}
		}
		assert.Equal(t, []string{confidential.Title, confidential.Title}, dependencyTitles)

		for _, list := range []string{"/dependencies", "/blocks"} {
			titles, total := apiIssueTitles(t, viewer.apiRequest(t, apiLink+list))
			assert.Equal(t, []string{confidential.Title}, titles, list)
			assert.Equal(t, "1", total, list)
		}
	}

	for _, viewer := range []confidentialViewer{"", "user5"} {
		t.Run("hidden "+string(viewer), func(t *testing.T) { checkHidden(t, viewer) })
	}
	for _, viewer := range []confidentialViewer{"user2", "user10", "user8"} {
		t.Run("allowed "+string(viewer), func(t *testing.T) { checkVisible(t, viewer) })
	}

	setConfidentialColumn(t, confidential, false)
	for _, viewer := range []confidentialViewer{"", "user5"} {
		t.Run("published "+string(viewer), func(t *testing.T) { checkVisible(t, viewer) })
	}
}
