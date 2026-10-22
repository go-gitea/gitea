// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/test"

	"github.com/google/go-github/v92/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGithubMissingCommentIssue(t *testing.T) {
	defer test.MockVariableValue(&setting.Migrations.MaxAttempts, 2)()
	defer test.MockVariableValue(&setting.Migrations.RetryBackoff, 0)()
	for _, scenario := range []string{"recover", "forbidden", "not found", "pull request", "wrong number", "wrong repository", "issues disabled"} {
		t.Run(scenario, func(t *testing.T) {
			unittest.PrepareTestEnv(t)
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-RateLimit-Remaining", "100")
				if strings.HasSuffix(r.URL.Path, "/reactions") {
					if r.URL.Query().Get("page") == "1" {
						_, _ = w.Write([]byte(`[{"user":{"id":2,"login":"reviewer"},"content":"+1"}]`))
					} else {
						_, _ = w.Write([]byte(`[]`))
					}
					return
				}
				assert.Equal(t, "/api/v3/repos/o/r/issues/4877", r.URL.Path)
				requests++
				if scenario == "recover" && requests == 1 {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				if scenario == "forbidden" || scenario == "not found" {
					status := http.StatusForbidden
					if scenario == "not found" {
						status = http.StatusNotFound
					}
					w.WriteHeader(status)
					return
				}
				issue := map[string]any{
					"number": 4877, "title": "Missing issue", "body": "Original body", "state": "closed",
					"repository_url": "http://" + r.Host + "/api/v3/repos/o/r",
					"user":           map[string]any{"id": 1, "login": "author"},
					"created_at":     "2024-08-12T08:38:46Z", "updated_at": "2025-01-21T10:05:11Z", "closed_at": "2025-01-21T10:05:11Z",
					"labels":    []any{map[string]any{"name": "bug"}},
					"assignees": []any{map[string]any{"login": "assignee"}},
					"milestone": map[string]any{"title": "v1"}, "reactions": map[string]any{"total_count": 1},
				}
				switch scenario {
				case "pull request":
					issue["pull_request"] = map[string]any{"url": "pulls/4877"}
				case "wrong number":
					issue["number"] = 4878
				case "wrong repository":
					issue["repository_url"] = "http://" + r.Host + "/api/v3/repos/other/repo"
				}
				assert.NoError(t, json.NewEncoder(w).Encode(issue))
			}))
			defer server.Close()
			downloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", "", "o", "r")
			require.NoError(t, err)
			local := NewGiteaLocalUploader(t.Context(), unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}), "user2", "repo1")
			local.repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
			local.gitServiceType = structs.GithubService
			require.NoError(t, local.CreateIssues(t.Context(), &base.Issue{Number: 4876, Title: "Existing"}))
			uploader := githubMigrationUploader(local, base.NewRetryDownloader(downloader, 2, 0), base.MigrateOptions{Issues: scenario != "issues disabled"})
			comments := []*base.Comment{{Index: 1, IssueIndex: 4876}, {Index: 2, IssueIndex: 4877}, {Index: 3, IssueIndex: 4877}}
			err = uploader.CreateComments(t.Context(), comments...)
			if scenario != "recover" {
				require.Error(t, err)
				assert.Zero(t, unittest.GetCount(t, &issues_model.Comment{IssueID: local.issues[4876].ID}))
				assert.Len(t, local.issues, 1)
				if scenario == "issues disabled" {
					assert.Zero(t, requests)
				}
				return
			}
			require.NoError(t, err)
			issue := local.issues[4877]
			require.NotNil(t, issue)
			assert.Equal(t, "Missing issue", issue.Title)
			assert.Equal(t, "Original body", issue.Content)
			assert.True(t, issue.IsClosed)
			assert.Equal(t, "2024-08-12T08:38:46Z", issue.CreatedUnix.AsTime().UTC().Format(time.RFC3339))
			assert.Equal(t, "2025-01-21T10:05:11Z", issue.ClosedUnix.AsTime().UTC().Format(time.RFC3339))
			assert.Equal(t, 1, unittest.GetCount(t, &issues_model.Comment{IssueID: local.issues[4876].ID}))
			assert.Equal(t, 2, unittest.GetCount(t, &issues_model.Comment{IssueID: issue.ID}))
			assert.Equal(t, 1, unittest.GetCount(t, &issues_model.Reaction{IssueID: issue.ID}))
			require.NoError(t, uploader.CreateComments(t.Context(), &base.Comment{Index: 4, IssueIndex: 4877}))
			assert.Equal(t, 2, requests)
			assert.Equal(t, 3, unittest.GetCount(t, &issues_model.Comment{IssueID: issue.ID}))
		})
	}
}

func TestGithubReactionRequests(t *testing.T) {
	for _, tc := range []struct {
		name      string
		reactions *github.Reactions
		skip      bool
		want      int
	}{
		{name: "missing", want: 1},
		{name: "missing total", reactions: &github.Reactions{}, want: 1},
		{name: "zero", reactions: &github.Reactions{TotalCount: new(0)}},
		{name: "nonzero", reactions: &github.Reactions{TotalCount: new(1)}, want: 1},
		{name: "multiple pages", reactions: &github.Reactions{TotalCount: new(2)}, want: 2},
		{name: "skip", reactions: &github.Reactions{TotalCount: new(1)}, skip: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal([]any{map[string]any{
				"id": 1, "number": 1, "title": "Issue", "user": map[string]any{},
				"issue_url": "https://api.github.com/repos/o/r/issues/1", "reactions": tc.reactions,
			}})
			require.NoError(t, err)
			reactionRequests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-RateLimit-Remaining", "100")
				if strings.HasSuffix(r.URL.Path, "/reactions") {
					reactionRequests++
					if tc.want == 2 && r.URL.Query().Get("page") == "1" {
						w.Header().Set("Link", "<http://"+r.Host+r.URL.Path+"?page=3>; rel=\"next\"")
					}
					if r.URL.Query().Get("page") == "1" || (tc.want == 2 && r.URL.Query().Get("page") == "3") {
						_, _ = w.Write([]byte(`[{"user":{"id":2,"login":"user"},"content":"+1"}]`))
					} else {
						_, _ = w.Write([]byte(`[]`))
					}
					return
				}
				_, _ = w.Write(payload)
			}))
			defer server.Close()
			downloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", "", "o", "r")
			require.NoError(t, err)
			downloader.SkipReactions = tc.skip
			archivedReactions := 0
			downloader.archive = func(_ context.Context, kind string, _ int64, value any) error {
				if kind == "reactions" {
					archivedReactions++
					reaction, ok := value.(*github.Reaction)
					require.True(t, ok)
					assert.Equal(t, "+1", reaction.GetContent())
				}
				return nil
			}
			issues, _, err := downloader.GetIssues(t.Context(), 1, 100)
			require.NoError(t, err)
			require.Len(t, issues, 1)
			comments, _, err := downloader.GetComments(t.Context(), &base.Issue{Number: 1, ForeignIndex: 1})
			require.NoError(t, err)
			require.Len(t, comments, 1)
			allComments, _, err := downloader.GetAllComments(t.Context(), 1, 100)
			require.NoError(t, err)
			require.Len(t, allComments, 1)
			reviewComments, err := downloader.convertGithubReviewComments(t.Context(), []*github.PullRequestComment{{ID: new(int64(1)), Reactions: tc.reactions}})
			require.NoError(t, err)
			require.Len(t, reviewComments, 1)
			for _, reactions := range [][]*base.Reaction{issues[0].Reactions, comments[0].Reactions, allComments[0].Reactions, reviewComments[0].Reactions} {
				require.Len(t, reactions, tc.want)
				if tc.want > 0 {
					assert.Equal(t, &base.Reaction{UserID: 2, UserName: "user", Content: "+1"}, reactions[0])
				}
			}
			assert.Equal(t, 4*tc.want, reactionRequests)
			before := reactionRequests
			prs, err := downloader.convertGithubPullRequests(t.Context(), []*github.PullRequest{{Number: new(1)}}, 100)
			require.NoError(t, err)
			wantPR := max(tc.want, 1)
			if tc.skip {
				wantPR = 0
			}
			require.Len(t, prs[0].Reactions, wantPR)
			assert.Equal(t, wantPR, reactionRequests-before)
			assert.Equal(t, 4*tc.want+wantPR, archivedReactions)
		})
	}
}

func TestGithubReviewCommentPagination(t *testing.T) {
	for _, failComments := range []bool{false, true} {
		t.Run(fmt.Sprintf("failComments=%t", failComments), func(t *testing.T) {
			var requests []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-RateLimit-Remaining", "100")
				requests = append(requests, r.URL.RequestURI())
				page := r.URL.Query().Get("page")
				switch r.URL.Path {
				case "/api/v3/repos/o/r/pulls/1/reviews":
					if page == "2" {
						_, _ = w.Write([]byte(`[{"id":30,"state":"APPROVED"}]`))
					} else {
						w.Header().Set("Link", `<https://api.github.com/repos/o/r/pulls/1/reviews?page=2>; rel="next"`)
						_, _ = w.Write([]byte(`[{"id":10,"state":"COMMENTED"},{"id":20,"state":"COMMENTED"}]`))
					}
				case "/api/v3/repos/o/r/pulls/1/comments":
					if page == "2" {
						if failComments {
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						_, _ = w.Write([]byte(`[{"id":2,"pull_request_review_id":10,"in_reply_to_id":1,"reactions":{"total_count":0}}]`))
					} else {
						w.Header().Set("Link", `<https://api.github.com/repos/o/r/pulls/1/comments?page=2>; rel="next"`)
						_, _ = w.Write([]byte(`[{"id":1,"pull_request_review_id":20,"reactions":{"total_count":0}}]`))
					}
				case "/api/v3/repos/o/r/pulls/1/requested_reviewers":
					_, _ = w.Write([]byte(`{"users":[{"id":42,"login":"reviewer"}]}`))
				default:
					t.Errorf("unexpected request: %s", r.URL)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			downloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", "", "o", "r")
			require.NoError(t, err)
			reviews, err := downloader.GetReviews(t.Context(), &base.PullRequest{Number: 7, ForeignIndex: 1})
			if failComments {
				require.Error(t, err)
				assert.Nil(t, reviews)
				return
			}
			require.NoError(t, err)
			require.Len(t, reviews, 4)
			require.Len(t, reviews[0].Comments, 1)
			assert.EqualValues(t, 2, reviews[0].Comments[0].ID)
			assert.EqualValues(t, 1, reviews[0].Comments[0].InReplyTo)
			require.Len(t, reviews[1].Comments, 1)
			assert.EqualValues(t, 1, reviews[1].Comments[0].ID)
			assert.Empty(t, reviews[2].Comments)
			assert.Equal(t, base.ReviewStateRequestReview, reviews[3].State)
			assert.EqualValues(t, 42, reviews[3].ReviewerID)
			for _, review := range reviews {
				assert.EqualValues(t, 7, review.IssueIndex)
			}
			assert.Equal(t, []string{
				"/api/v3/repos/o/r/pulls/1/reviews?per_page=100",
				"/api/v3/repos/o/r/pulls/1/reviews?page=2&per_page=100",
				"/api/v3/repos/o/r/pulls/1/comments?per_page=100",
				"/api/v3/repos/o/r/pulls/1/comments?page=2&per_page=100",
				"/api/v3/repos/o/r/pulls/1/requested_reviewers",
			}, requests)
		})
	}
}

func TestGithubRecoverMissingReview(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprintf("missing=%t", missing), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-RateLimit-Remaining", "100")
				switch r.URL.Path {
				case "/api/v3/repos/o/r/pulls/1/reviews":
					_, _ = io.WriteString(w, `[]`)
				case "/api/v3/repos/o/r/pulls/1/comments":
					_, _ = io.WriteString(w, `[{"id":1,"pull_request_review_id":10,"path":"file.go","line":4,"side":"LEFT","start_line":2,"reactions":{"total_count":0}}]`)
				case "/api/v3/repos/o/r/pulls/1/reviews/10":
					if missing {
						w.WriteHeader(http.StatusNotFound)
					} else {
						_, _ = io.WriteString(w, `{"id":10,"state":"COMMENTED"}`)
					}
				case "/api/v3/repos/o/r/pulls/1/requested_reviewers":
					_, _ = io.WriteString(w, `{"users":[]}`)
				default:
					t.Errorf("unexpected URL %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			downloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", "", "o", "r")
			require.NoError(t, err)
			archived := make(map[string]any)
			downloader.archive = func(_ context.Context, kind string, _ int64, value any) error { archived[kind] = value; return nil }
			reviews, err := downloader.GetReviews(t.Context(), &base.PullRequest{Number: 1, ForeignIndex: 1})
			if missing {
				require.ErrorContains(t, err, "recover review 10")
				assert.Nil(t, reviews)
				return
			}
			require.NoError(t, err)
			require.Len(t, reviews, 1)
			require.Len(t, reviews[0].Comments, 1)
			assert.Equal(t, -4, reviews[0].Comments[0].Line)
			source, ok := archived["review-comments"].(*github.PullRequestComment)
			require.True(t, ok)
			assert.Equal(t, 2, source.GetStartLine())
			assert.Equal(t, "LEFT", source.GetSide())
		})
	}
}

func TestGithubOutdatedCommentPosition(t *testing.T) {
	downloader := &GithubDownloaderV3{SkipReactions: true}
	comments, err := downloader.convertGithubReviewComments(t.Context(), []*github.PullRequestComment{{ID: new(int64(1)), OriginalLine: new(12), OriginalPosition: new(3), Side: new("LEFT"), CommitID: new("current"), OriginalCommitID: new("original")}})
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, -12, comments[0].Line)
	assert.Equal(t, "original", comments[0].CommitID)
	assert.Equal(t, 3, comments[0].Position)
}

func TestGithubIssuePagination(t *testing.T) {
	for _, tc := range []struct {
		name  string
		pages []string
	}{
		{name: "offset", pages: []string{"page=1", "page=2", "page=3"}},
		{name: "cursor", pages: []string{"page=1", "after=cursor-2", "after=cursor-3"}},
		{name: "switch to cursor", pages: []string{"page=1", "page=2", "after=cursor-3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests []string
			failReaction := true
			issue := func(number int) string {
				return fmt.Sprintf(`{"number":%d,"title":"Issue","user":{},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`, number)
			}
			responses := []string{
				"[" + issue(1) + `,{"pull_request":{}}]`,
				`[{"pull_request":{}}]`,
				"[" + issue(4) + "," + issue(5) + "]",
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-RateLimit-Remaining", "100")
				if r.URL.Path == "/api/v3/repos/o/r/issues/4/reactions" && failReaction {
					failReaction = false
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"message":"temporary failure"}`))
					return
				}
				if r.URL.Path != "/api/v3/repos/o/r/issues" {
					_, _ = w.Write([]byte(`[]`))
					return
				}
				query := r.URL.Query()
				assert.Equal(t, "2", query.Get("per_page"))
				assert.Equal(t, "all", query.Get("state"))
				assert.Equal(t, "created", query.Get("sort"))
				assert.Equal(t, "asc", query.Get("direction"))
				query.Del("per_page")
				query.Del("state")
				query.Del("sort")
				query.Del("direction")
				pagination := query.Encode()
				requests = append(requests, pagination)
				page := slices.Index(tc.pages, pagination)
				if page < 0 {
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, _ = w.Write([]byte(`{"message":"unexpected pagination"}`))
					return
				}
				if page+1 < len(tc.pages) {
					w.Header().Set("Link", fmt.Sprintf(`<https://api.github.com/repos/o/r/issues?per_page=2&%s>; rel="next"`, tc.pages[page+1]))
				}
				_, _ = w.Write([]byte(responses[page]))
			}))
			defer server.Close()

			downloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", "", "o", "r")
			require.NoError(t, err)
			issues, isEnd, err := downloader.GetIssues(t.Context(), 1, 2)
			require.NoError(t, err)
			require.Len(t, issues, 1)
			assert.EqualValues(t, 1, issues[0].Number)
			assert.False(t, isEnd)

			for range 2 {
				issues, isEnd, err = downloader.GetIssues(t.Context(), 2, 2)
				require.NoError(t, err)
				assert.Empty(t, issues)
				assert.False(t, isEnd)
			}

			_, _, err = downloader.GetIssues(t.Context(), 3, 2)
			require.Error(t, err)
			for range 2 {
				issues, isEnd, err = downloader.GetIssues(t.Context(), 3, 2)
				require.NoError(t, err)
				require.Len(t, issues, 2)
				assert.EqualValues(t, 4, issues[0].Number)
				assert.EqualValues(t, 5, issues[1].Number)
				assert.True(t, isEnd)
			}

			_, isEnd, err = downloader.GetIssues(t.Context(), 1, 2)
			require.NoError(t, err)
			assert.False(t, isEnd)
			issues, isEnd, err = downloader.GetIssues(t.Context(), 2, 2)
			require.NoError(t, err)
			assert.Empty(t, issues)
			assert.False(t, isEnd)
			assert.Equal(t, []string{tc.pages[0], tc.pages[1], tc.pages[1], tc.pages[2], tc.pages[2], tc.pages[2], tc.pages[0], tc.pages[1]}, requests)
		})
	}
}

func TestGitHubDownloadRepo(t *testing.T) {
	token := os.Getenv("GITHUB_READ_TOKEN")
	liveMode := token != ""

	_, callerFile, _, _ := runtime.Caller(0)
	fixtureDir := filepath.Join(filepath.Dir(callerFile), "_mock_data/TestGitHubDownloadRepo")
	mockServer := unittest.NewMockWebServer(t, "https://api.github.com", fixtureDir, liveMode, unittest.MockServerOptions{
		StripPrefix: "/api/v3",
	})

	GithubLimitRateRemaining = 3 // Wait at 3 remaining since we could have 3 CI in //
	ctx := t.Context()
	downloader, err := NewGithubDownloaderV3(ctx, mockServer.URL, "", "", token, "go-gitea", "test_repo")
	require.NoError(t, err)
	err = downloader.RefreshRate(ctx)
	require.NoError(t, err)

	repo, err := downloader.GetRepoInfo(ctx)
	assert.NoError(t, err)
	assertRepositoryEqual(t, &base.Repository{
		Name:          "test_repo",
		Owner:         "go-gitea",
		Description:   "Test repository for testing migration from github to gitea",
		Website:       "https://gitea.com/test-repo",
		CloneURL:      "https://github.com/go-gitea/test_repo.git",
		OriginalURL:   "https://github.com/go-gitea/test_repo",
		DefaultBranch: "master",
	}, repo)

	topics, err := downloader.GetTopics(ctx)
	assert.NoError(t, err)
	assert.Contains(t, topics, "gitea")

	milestones, err := downloader.GetMilestones(ctx)
	assert.NoError(t, err)
	assertMilestonesEqual(t, []*base.Milestone{
		{
			Title:       "1.0.0",
			Description: "Milestone 1.0.0",
			Deadline:    new(time.Date(2019, 11, 11, 0, 0, 0, 0, time.UTC)),
			Created:     time.Date(2019, 11, 12, 19, 37, 8, 0, time.UTC),
			Updated:     new(time.Date(2019, 11, 12, 21, 56, 17, 0, time.UTC)),
			Closed:      new(time.Date(2019, 11, 12, 19, 45, 49, 0, time.UTC)),
			State:       "closed",
		},
		{
			Title:       "1.1.0",
			Description: "Milestone 1.1.0",
			Deadline:    new(time.Date(2019, 11, 12, 0, 0, 0, 0, time.UTC)),
			Created:     time.Date(2019, 11, 12, 19, 37, 25, 0, time.UTC),
			Updated:     new(time.Date(2019, 11, 12, 21, 39, 27, 0, time.UTC)),
			Closed:      new(time.Date(2019, 11, 12, 19, 45, 46, 0, time.UTC)),
			State:       "closed",
		},
	}, milestones)

	labels, err := downloader.GetLabels(ctx)
	assert.NoError(t, err)
	assertLabelsEqual(t, []*base.Label{
		{
			Name:        "bug",
			Color:       "d73a4a",
			Description: "Something isn't working",
		},
		{
			Name:        "documentation",
			Color:       "0075ca",
			Description: "Improvements or additions to documentation",
		},
		{
			Name:        "duplicate",
			Color:       "cfd3d7",
			Description: "This issue or pull request already exists",
		},
		{
			Name:        "enhancement",
			Color:       "a2eeef",
			Description: "New feature or request",
		},
		{
			Name:        "good first issue",
			Color:       "7057ff",
			Description: "Good for newcomers",
		},
		{
			Name:        "help wanted",
			Color:       "008672",
			Description: "Extra attention is needed",
		},
		{
			Name:        "invalid",
			Color:       "e4e669",
			Description: "This doesn't seem right",
		},
		{
			Name:        "question",
			Color:       "d876e3",
			Description: "Further information is requested",
		},
		{
			Name:        "wontfix",
			Color:       "ffffff",
			Description: "This will not be worked on",
		},
	}, labels)

	releases, err := downloader.GetReleases(ctx)
	assert.NoError(t, err)
	assertReleasesEqual(t, []*base.Release{
		{
			TagName:         "v0.9.99",
			TargetCommitish: "master",
			Name:            "First Release",
			Body:            "A test release",
			Created:         time.Date(2019, 11, 9, 16, 49, 21, 0, time.UTC),
			Published:       time.Date(2019, 11, 12, 20, 12, 10, 0, time.UTC),
			PublisherID:     1669571,
			PublisherName:   "mrsdizzie",
		},
	}, releases)

	// downloader.GetIssues()
	issues, isEnd, err := downloader.GetIssues(ctx, 1, 2)
	assert.NoError(t, err)
	assert.False(t, isEnd)
	assertIssuesEqual(t, []*base.Issue{
		{
			Number:     1,
			Title:      "Please add an animated gif icon to the merge button",
			Content:    "I just want the merge button to hurt my eyes a little. \xF0\x9F\x98\x9D ",
			Milestone:  "1.0.0",
			PosterID:   18600385,
			PosterName: "guillep2k",
			State:      "closed",
			Created:    time.Date(2019, 11, 9, 17, 0, 29, 0, time.UTC),
			Updated:    time.Date(2019, 11, 12, 20, 29, 53, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:        "bug",
					Color:       "d73a4a",
					Description: "Something isn't working",
				},
				{
					Name:        "good first issue",
					Color:       "7057ff",
					Description: "Good for newcomers",
				},
			},
			Reactions: []*base.Reaction{
				{
					UserID:   1669571,
					UserName: "mrsdizzie",
					Content:  "+1",
				},
			},
			Closed: new(time.Date(2019, 11, 12, 20, 22, 22, 0, time.UTC)),
		},
		{
			Number:     2,
			Title:      "Test issue",
			Content:    "This is test issue 2, do not touch!",
			Milestone:  "1.1.0",
			PosterID:   1669571,
			PosterName: "mrsdizzie",
			State:      "closed",
			Created:    time.Date(2019, 11, 12, 21, 0, 6, 0, time.UTC),
			Updated:    time.Date(2019, 11, 12, 22, 7, 14, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:        "duplicate",
					Color:       "cfd3d7",
					Description: "This issue or pull request already exists",
				},
			},
			Reactions: []*base.Reaction{
				{
					UserID:   1669571,
					UserName: "mrsdizzie",
					Content:  "heart",
				},
				{
					UserID:   1669571,
					UserName: "mrsdizzie",
					Content:  "laugh",
				},
				{
					UserID:   1669571,
					UserName: "mrsdizzie",
					Content:  "-1",
				},
				{
					UserID:   1669571,
					UserName: "mrsdizzie",
					Content:  "confused",
				},
				{
					UserID:   1669571,
					UserName: "mrsdizzie",
					Content:  "hooray",
				},
				{
					UserID:   1669571,
					UserName: "mrsdizzie",
					Content:  "+1",
				},
			},
			Closed: new(time.Date(2019, 11, 12, 21, 1, 31, 0, time.UTC)),
		},
	}, issues)

	// downloader.GetComments()
	comments, _, err := downloader.GetComments(ctx, &base.Issue{Number: 2, ForeignIndex: 2})
	assert.NoError(t, err)
	assertCommentsEqual(t, []*base.Comment{
		{
			IssueIndex: 2,
			PosterID:   1669571,
			PosterName: "mrsdizzie",
			Created:    time.Date(2019, 11, 12, 21, 0, 13, 0, time.UTC),
			Updated:    time.Date(2019, 11, 12, 21, 0, 13, 0, time.UTC),
			Content:    "This is a comment",
			Reactions: []*base.Reaction{
				{
					UserID:   1669571,
					UserName: "mrsdizzie",
					Content:  "+1",
				},
			},
		},
		{
			IssueIndex: 2,
			PosterID:   1669571,
			PosterName: "mrsdizzie",
			Created:    time.Date(2019, 11, 12, 22, 7, 14, 0, time.UTC),
			Updated:    time.Date(2019, 11, 12, 22, 7, 14, 0, time.UTC),
			Content:    "A second comment",
			Reactions:  nil,
		},
	}, comments)

	// downloader.GetPullRequests()
	prs, _, err := downloader.GetPullRequests(ctx, 1, 2)
	assert.NoError(t, err)
	assertPullRequestsEqual(t, []*base.PullRequest{
		{
			Number:     3,
			Title:      "Update README.md",
			Content:    "add warning to readme",
			Milestone:  "1.1.0",
			PosterID:   1669571,
			PosterName: "mrsdizzie",
			State:      "closed",
			Created:    time.Date(2019, 11, 12, 21, 21, 43, 0, time.UTC),
			Updated:    time.Date(2019, 11, 12, 21, 39, 28, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:        "documentation",
					Color:       "0075ca",
					Description: "Improvements or additions to documentation",
				},
			},
			PatchURL: "",
			Head: base.PullRequestBranch{
				Ref:      "master",
				CloneURL: "",
				SHA:      "076160cf0b039f13e5eff19619932d181269414b",
				RepoName: "test_repo",

				OwnerName: "mrsdizzie",
			},
			Base: base.PullRequestBranch{
				Ref:       "master",
				SHA:       "72866af952e98d02a73003501836074b286a78f6",
				OwnerName: "go-gitea",
				RepoName:  "test_repo",
			},
			Closed:         new(time.Date(2019, 11, 12, 21, 39, 27, 0, time.UTC)),
			Merged:         true,
			MergedTime:     new(time.Date(2019, 11, 12, 21, 39, 27, 0, time.UTC)),
			MergeCommitSHA: "f32b0a9dfd09a60f616f29158f772cedd89942d2",
			ForeignIndex:   3,
		},
		{
			Number:     4,
			Title:      "Test branch",
			Content:    "do not merge this PR",
			Milestone:  "1.0.0",
			PosterID:   1669571,
			PosterName: "mrsdizzie",
			State:      "open",
			Created:    time.Date(2019, 11, 12, 21, 54, 18, 0, time.UTC),
			Updated:    time.Date(2025, 3, 16, 15, 46, 20, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:        "bug",
					Color:       "d73a4a",
					Description: "Something isn't working",
				},
			},
			PatchURL: "",
			Head: base.PullRequestBranch{
				Ref:       "test-branch",
				SHA:       "2be9101c543658591222acbee3eb799edfc3853d",
				RepoName:  "test_repo",
				OwnerName: "mrsdizzie",
				CloneURL:  "",
			},
			Base: base.PullRequestBranch{
				Ref:       "master",
				SHA:       "f32b0a9dfd09a60f616f29158f772cedd89942d2",
				OwnerName: "go-gitea",
				RepoName:  "test_repo",
			},
			Merged:         false,
			MergeCommitSHA: "565d1208f5fffdc1c5ae1a2436491eb9a5e4ebae",
			Reactions: []*base.Reaction{
				{
					UserID:   81045,
					UserName: "lunny",
					Content:  "heart",
				},
				{
					UserID:   81045,
					UserName: "lunny",
					Content:  "+1",
				},
			},
			ForeignIndex: 4,
		},
	}, prs)

	reviews, err := downloader.GetReviews(ctx, &base.PullRequest{Number: 3, ForeignIndex: 3})
	assert.NoError(t, err)
	assertReviewsEqual(t, []*base.Review{
		{
			ID:           315859956,
			IssueIndex:   3,
			ReviewerID:   42128690,
			ReviewerName: "jolheiser",
			CommitID:     "076160cf0b039f13e5eff19619932d181269414b",
			CreatedAt:    time.Date(2019, 11, 12, 21, 35, 24, 0, time.UTC),
			State:        base.ReviewStateApproved,
		},
		{
			ID:           315860062,
			IssueIndex:   3,
			ReviewerID:   1824502,
			ReviewerName: "zeripath",
			CommitID:     "076160cf0b039f13e5eff19619932d181269414b",
			CreatedAt:    time.Date(2019, 11, 12, 21, 35, 36, 0, time.UTC),
			State:        base.ReviewStateApproved,
		},
		{
			ID:           315861440,
			IssueIndex:   3,
			ReviewerID:   165205,
			ReviewerName: "lafriks",
			CommitID:     "076160cf0b039f13e5eff19619932d181269414b",
			CreatedAt:    time.Date(2019, 11, 12, 21, 38, 0, 0, time.UTC),
			State:        base.ReviewStateApproved,
		},
	}, reviews)

	reviews, err = downloader.GetReviews(ctx, &base.PullRequest{Number: 4, ForeignIndex: 4})
	assert.NoError(t, err)
	assertReviewsEqual(t, []*base.Review{
		{
			ID:           338338740,
			IssueIndex:   4,
			ReviewerID:   81045,
			ReviewerName: "lunny",
			CommitID:     "2be9101c543658591222acbee3eb799edfc3853d",
			CreatedAt:    time.Date(2020, 1, 4, 5, 33, 18, 0, time.UTC),
			State:        base.ReviewStateApproved,
			Comments: []*base.ReviewComment{
				{
					ID:        363017488,
					Content:   "This is a good pull request.",
					TreePath:  "README.md",
					DiffHunk:  "@@ -1,2 +1,4 @@\n # test_repo\n Test repository for testing migration from github to gitea\n+",
					Position:  3,
					CommitID:  "2be9101c543658591222acbee3eb799edfc3853d",
					PosterID:  81045,
					CreatedAt: time.Date(2020, 1, 4, 5, 33, 6, 0, time.UTC),
					UpdatedAt: time.Date(2020, 1, 4, 5, 33, 18, 0, time.UTC),
				},
			},
		},
		{
			ID:           338339651,
			IssueIndex:   4,
			ReviewerID:   81045,
			ReviewerName: "lunny",
			CommitID:     "2be9101c543658591222acbee3eb799edfc3853d",
			CreatedAt:    time.Date(2020, 1, 4, 6, 7, 6, 0, time.UTC),
			State:        base.ReviewStateChangesRequested,
			Content:      "Don't add more reviews",
		},
		{
			ID:           338349019,
			IssueIndex:   4,
			ReviewerID:   81045,
			ReviewerName: "lunny",
			CommitID:     "2be9101c543658591222acbee3eb799edfc3853d",
			CreatedAt:    time.Date(2020, 1, 4, 11, 21, 41, 0, time.UTC),
			State:        base.ReviewStateCommented,
			Comments: []*base.ReviewComment{
				{
					ID:        363029944,
					Content:   "test a single comment.",
					TreePath:  "LICENSE",
					DiffHunk:  "@@ -19,3 +19,5 @@ AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER\n LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,\n OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE\n SOFTWARE.\n+",
					Position:  4,
					CommitID:  "2be9101c543658591222acbee3eb799edfc3853d",
					PosterID:  81045,
					CreatedAt: time.Date(2020, 1, 4, 11, 21, 41, 0, time.UTC),
					UpdatedAt: time.Date(2020, 1, 4, 11, 21, 41, 0, time.UTC),
				},
			},
		},
	}, reviews)
}

func TestConvertGithubDismissedReview(t *testing.T) {
	review := convertGithubReview(&github.PullRequestReview{State: new("DISMISSED")})
	assert.Equal(t, base.ReviewStateCommented, review.State)
	assert.True(t, review.Dismissed)
}

func TestGithubFormatCloneURL(t *testing.T) {
	testCases := []struct {
		desc             string
		token            string
		username         string
		expectedCloneURL string
	}{
		{
			desc:             "Single Token",
			token:            "single_token",
			expectedCloneURL: "https://oauth2:single_token@github.com",
		},
		{
			desc:             "Multi Token",
			token:            "token1,token2",
			expectedCloneURL: "https://oauth2:token1@github.com",
		},
		{
			desc:             "Username And Password",
			username:         "user",
			expectedCloneURL: "https://user:password@github.com",
		},
	}
	factory := GithubDownloaderV3Factory{}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			opts := base.MigrateOptions{CloneAddr: "https://github.com/go-gitea/gitea", AuthToken: tC.token, AuthUsername: tC.username, AuthPassword: "password"}
			client, err := factory.New(t.Context(), opts)
			require.NoError(t, err)

			cloneURL, err := client.FormatCloneURL(opts, "https://github.com")
			require.NoError(t, err)

			assert.Equal(t, tC.expectedCloneURL, cloneURL)
		})
	}
}

func TestGithubMultiTokenClientSelection(t *testing.T) {
	downloader := &GithubDownloaderV3{
		clients: make([]*github.Client, 3),
		rates:   make([]*github.Rate, 3),
	}

	downloader.waitAndPickClient(t.Context())
	assert.Equal(t, 0, downloader.curClientIdx)

	downloader.rates[0] = &github.Rate{Remaining: 100}
	downloader.waitAndPickClient(t.Context())
	assert.Equal(t, 1, downloader.curClientIdx)

	downloader.rates[1] = &github.Rate{Remaining: 200}
	downloader.waitAndPickClient(t.Context())
	assert.Equal(t, 2, downloader.curClientIdx)

	downloader.rates[2] = &github.Rate{Remaining: 50}
	downloader.waitAndPickClient(t.Context())
	assert.Equal(t, 1, downloader.curClientIdx)
}

func TestGitHubIssuesPageBasedLinkFallback(t *testing.T) {
	_, callerFile, _, _ := runtime.Caller(0)
	mockServer := unittest.NewMockWebServer(t, "https://api.github.com", filepath.Join(filepath.Dir(callerFile), "_mock_data/TestGitHubDownloadRepo"), false, unittest.MockServerOptions{
		StripPrefix: "/api/v3",
	})
	downloader, err := NewGithubDownloaderV3(t.Context(), mockServer.URL, "", "", "", "go-gitea", "test_repo")
	require.NoError(t, err)
	downloader.SkipReactions = true
	for page, want := range [][]int64{{1, 2, 3}, {}} {
		issues, isEnd, err := downloader.GetIssues(t.Context(), page+1, 3)
		require.NoError(t, err)
		numbers := make([]int64, 0, len(issues))
		for _, issue := range issues {
			numbers = append(numbers, issue.Number)
		}
		assert.Equal(t, want, numbers)
		assert.Equal(t, page == 1, isEnd)
	}
}

func TestGithubFactoryUsesGraphQLOnlyWhereItWorks(t *testing.T) {
	for _, testcase := range []struct {
		name string
		opts base.MigrateOptions
		want bool
	}{
		{name: "token, issues", opts: base.MigrateOptions{AuthToken: "tok", Issues: true}, want: true},
		{name: "basic auth, pull requests", opts: base.MigrateOptions{AuthUsername: "u", AuthPassword: "pat", PullRequests: true}, want: true},
		{name: "anonymous repository", opts: base.MigrateOptions{Issues: true, PullRequests: true}},
		{name: "comments only", opts: base.MigrateOptions{AuthToken: "tok", Comments: true}},
		{name: "issues with comments", opts: base.MigrateOptions{AuthToken: "tok", Issues: true, Comments: true}, want: true},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			testcase.opts.CloneAddr = "https://github.com/go-gitea/test_repo"
			candidate, err := (&GithubDownloaderV3Factory{}).New(t.Context(), testcase.opts)
			require.NoError(t, err)
			downloader, isGraphQL := candidate.(*GithubGraphQLDownloader)
			assert.Equal(t, testcase.want, isGraphQL)
			if isGraphQL {
				assert.Equal(t, !testcase.opts.Comments, downloader.skipComments)
			}
		})
	}
}

func TestGithubGraphQLFallsBackToREST(t *testing.T) {
	for _, connection := range []string{"issues", "pull requests"} {
		t.Run(connection, func(t *testing.T) {
			var restPage string
			downloader := newGithubTestDownloader(t, func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/api/graphql" {
					http.NotFound(writer, request)
					return
				}
				restPage = request.URL.Query().Get("page")
				_, _ = writer.Write([]byte(`[{"number":1,"title":"t","state":"open"}]`))
			})
			downloader.SkipReactions = true
			if connection == "issues" {
				issues, _, err := downloader.GetIssues(t.Context(), 3, 10)
				require.NoError(t, err)
				require.Len(t, issues, 1)
				assert.EqualValues(t, 1, issues[0].Number)
			} else {
				prs, _, err := downloader.GetPullRequests(t.Context(), 3, 10)
				require.NoError(t, err)
				require.Len(t, prs, 1)
				assert.EqualValues(t, 1, prs[0].Number)
			}
			assert.True(t, downloader.restFallback)
			assert.Equal(t, "1", restPage)
		})
	}
}

func TestGitBucketPaginatesWithoutLinkHeader(t *testing.T) {
	const total = 26
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		page, _ := strconv.Atoi(request.URL.Query().Get("page"))
		var items []string
		for number := (page-1)*25 + 1; number <= min(page*25, total); number++ {
			items = append(items, fmt.Sprintf(`{"number":%d,"title":"t"}`, number))
		}
		writer.Header().Set("X-RateLimit-Remaining", "4000")
		_, _ = writer.Write([]byte("[" + strings.Join(items, ",") + "]"))
	}))
	t.Cleanup(server.Close)
	downloader, err := NewGitBucketDownloader(t.Context(), server.URL, "", "", "", "o", "r")
	require.NoError(t, err)
	for _, connection := range []string{"issues", "pull requests"} {
		t.Run(connection, func(t *testing.T) {
			var count int
			for page := 1; ; page++ {
				require.LessOrEqual(t, page, 3)
				var isEnd bool
				if connection == "issues" {
					issues, done, err := downloader.GetIssues(t.Context(), page, 10)
					require.NoError(t, err)
					count += len(issues)
					isEnd = done
				} else {
					prs, done, err := downloader.GetPullRequests(t.Context(), page, 10)
					require.NoError(t, err)
					count += len(prs)
					isEnd = done
				}
				if isEnd {
					break
				}
			}
			assert.Equal(t, total, count)
		})
	}
}
