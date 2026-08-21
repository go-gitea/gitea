// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	system_model "gitea.dev/models/system"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/json"
	base "gitea.dev/modules/migration"

	"github.com/google/go-github/v89/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubDownloadRepo(t *testing.T) {
	token := os.Getenv("GITHUB_READ_TOKEN")
	liveMode := token != ""

	_, callerFile, _, _ := runtime.Caller(0)
	fixtureDir := filepath.Join(filepath.Dir(callerFile), "_mock_data/TestGitHubDownloadRepo")
	mockServer := unittest.NewMockWebServer(t, "https://api.github.com", fixtureDir, liveMode, unittest.MockServerOptions{
		StripPrefix: "/api/v3",
		Routes: func(mux *http.ServeMux) {
			mux.HandleFunc("/api/graphql", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[],"pageInfo":{"hasNextPage":false}}}}}}`))
			})
		},
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
	assert.Equal(t, time.Date(2019, 11, 12, 20, 22, 13, 0, time.UTC), issues[0].Reactions[0].Created)
	assert.Equal(t, &base.ExternalUser{ID: 1669571, Name: "mrsdizzie"}, issues[0].ClosedBy)
	assert.Equal(t, "completed", issues[0].CloseReason)

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
	assert.Equal(t, time.Date(2019, 11, 12, 21, 13, 22, 0, time.UTC), comments[0].Reactions[0].Created)

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
	assert.Equal(t, time.Date(2020, 1, 10, 8, 31, 30, 0, time.UTC), prs[1].Reactions[0].Created)

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
					ID:         363017488,
					Content:    "This is a good pull request.",
					TreePath:   "README.md",
					DiffHunk:   "@@ -1,2 +1,4 @@\n # test_repo\n Test repository for testing migration from github to gitea\n+",
					Position:   3,
					Line:       3,
					CommitID:   "2be9101c543658591222acbee3eb799edfc3853d",
					PosterID:   81045,
					PosterName: "lunny",
					CreatedAt:  time.Date(2020, 1, 4, 5, 33, 6, 0, time.UTC),
					UpdatedAt:  time.Date(2020, 1, 4, 5, 33, 18, 0, time.UTC),
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
					ID:         363029944,
					Content:    "test a single comment.",
					TreePath:   "LICENSE",
					DiffHunk:   "@@ -19,3 +19,5 @@ AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER\n LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,\n OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE\n SOFTWARE.\n+",
					Position:   4,
					Line:       22,
					CommitID:   "2be9101c543658591222acbee3eb799edfc3853d",
					PosterID:   81045,
					PosterName: "lunny",
					CreatedAt:  time.Date(2020, 1, 4, 11, 21, 41, 0, time.UTC),
					UpdatedAt:  time.Date(2020, 1, 4, 11, 21, 41, 0, time.UTC),
				},
			},
		},
	}, reviews)
}

func TestGithubPullRequestLockedWithoutReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v3/repos/owner/repo/pulls" {
			http.Error(w, "unexpected request: "+r.URL.String(), http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`[{
  "number":1,"state":"closed","locked":true,"active_lock_reason":null,
  "title":"locked","user":{"login":"author","id":1},"body":"",
  "created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","closed_at":"2026-01-01T00:00:00Z",
  "head":{"ref":"topic","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","user":{"login":"author","id":1},"repo":{"name":"repo","clone_url":"https://example.com/author/repo.git"}},
  "base":{"ref":"main","sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","user":{"login":"owner","id":2},"repo":{"name":"repo"}}
}]`))
	}))
	t.Cleanup(server.Close)

	downloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", "", "owner", "repo")
	require.NoError(t, err)
	downloader.SkipReactions = true
	prs, _, err := downloader.GetPullRequests(t.Context(), 1, 1)
	require.NoError(t, err)
	if assert.Len(t, prs, 1) {
		assert.True(t, prs[0].IsLocked)
	}
}

func TestGithubUserMetadataFromExistingLists(t *testing.T) {
	noticesBefore := system_model.CountNotices(t.Context())
	issueRequests := 0
	pullRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/repos/owner/repo/issues":
			issueRequests++
			_, _ = w.Write([]byte(`[
  {"number":1,"state":"closed","state_reason":"not_planned","locked":false,"title":"issue","body":"","user":{"login":"author","id":10},"closed_by":{"login":"closer","id":20},"assignees":[{"login":"assigned","id":30}],"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T00:00:00Z","closed_at":"2026-01-02T00:00:00Z"},
  {"number":2,"state":"closed","state_reason":"completed","locked":false,"title":"pull","body":"","user":{"login":"author","id":10},"closed_by":{"login":"merger","id":40},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T00:00:00Z","closed_at":"2026-01-02T00:00:00Z","pull_request":{}}
]`))
		case "/api/v3/repos/owner/repo/pulls":
			pullRequests++
			_, _ = w.Write([]byte(`[{
  "number":2,"state":"closed","locked":false,"title":"pull","body":"","user":{"login":"author","id":10},
  "assignees":[{"login":"assigned","id":30}],"requested_reviewers":[{"login":"requested","id":50}],
  "created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T00:00:00Z","closed_at":"2026-01-02T00:00:00Z","merged_at":"2026-01-02T00:00:00Z",
  "head":{"ref":"topic","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","user":{"login":"author","id":10},"repo":{"name":"repo"}},
  "base":{"ref":"main","sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","user":{"login":"owner","id":60},"repo":{"name":"repo"}}
}]`))
		default:
			http.Error(w, "unexpected request: "+r.URL.String(), http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	downloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", "", "owner", "repo")
	require.NoError(t, err)
	downloader.SkipReactions = true
	issues, _, err := downloader.GetIssues(t.Context(), 1, 100)
	require.NoError(t, err)
	if assert.Len(t, issues, 1) {
		assert.Equal(t, &base.ExternalUser{ID: 20, Name: "closer"}, issues[0].ClosedBy)
		assert.Equal(t, "not_planned", issues[0].CloseReason)
		assert.Equal(t, []*base.ExternalUser{{ID: 30, Name: "assigned"}}, issues[0].AssigneeUsers)
	}

	prs, _, err := downloader.GetPullRequests(t.Context(), 1, 100)
	require.NoError(t, err)
	if assert.Len(t, prs, 1) {
		assert.Equal(t, &base.ExternalUser{ID: 40, Name: "merger"}, prs[0].ClosedBy)
		assert.Equal(t, &base.ExternalUser{ID: 40, Name: "merger"}, prs[0].MergedBy)
		assert.Equal(t, "completed", prs[0].CloseReason)
		assert.Equal(t, []*base.ExternalUser{{ID: 30, Name: "assigned"}}, prs[0].AssigneeUsers)
		assert.Equal(t, githubPullRequestContext{requestedReviewers: []*base.ExternalUser{{ID: 50, Name: "requested"}}}, prs[0].Context)
	}

	prOnlyDownloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", "", "owner", "repo")
	require.NoError(t, err)
	prOnlyDownloader.SkipReactions = true
	prs, _, err = prOnlyDownloader.GetPullRequests(t.Context(), 1, 100)
	require.NoError(t, err)
	if assert.Len(t, prs, 1) {
		assert.Nil(t, prs[0].ClosedBy)
		assert.Nil(t, prs[0].MergedBy)
		assert.Empty(t, prs[0].CloseReason)
	}
	firstPRs := prs
	prs, _, err = prOnlyDownloader.GetPullRequests(t.Context(), 1, 100)
	require.NoError(t, err)
	assert.Equal(t, firstPRs, prs)
	assert.Equal(t, noticesBefore+1, system_model.CountNotices(t.Context()))
	assert.Equal(t, 1, issueRequests)
	assert.Equal(t, 3, pullRequests)
}

func TestConvertGithubDismissedReview(t *testing.T) {
	review := convertGithubReview(&github.PullRequestReview{State: new("DISMISSED")})
	assert.Equal(t, base.ReviewStateCommented, review.State)
}

func TestGroupGithubReviewCommentsReplyBeforeParent(t *testing.T) {
	reviews := map[int64]*base.Review{20: {ID: 20}, 30: {ID: 30}}
	reply := &base.ReviewComment{ID: 101, InReplyTo: 100}
	root := &base.ReviewComment{ID: 100, TreePath: "a.go", Line: 4, CommitID: "commit"}
	synthetic, vacated := groupGithubReviewComments(reviews, []githubReviewComment{
		{reviewID: 30, comment: reply},
		{reviewID: 20, comment: root},
	}, 7)

	assert.Empty(t, synthetic)
	assert.True(t, vacated[30])
	assert.Equal(t, []*base.ReviewComment{root, reply}, reviews[20].Comments)
	assert.Equal(t, root.TreePath, reply.TreePath)
	assert.Equal(t, root.Line, reply.Line)
	assert.Equal(t, root.CommitID, reply.CommitID)
}

func TestGithubReviewConversationMigration(t *testing.T) {
	graphqlRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Limit", "5000")
		w.Header().Set("X-RateLimit-Remaining", "4999")
		w.Header().Set("X-RateLimit-Reset", "2000000000")
		var body string
		switch r.URL.Path {
		case "/api/v3/repos/owner/repo/pulls/7/reviews":
			body = `[
  {"id":10,"user":{"login":"empty","id":10},"body":"","state":"COMMENTED","submitted_at":"2026-01-01T00:00:00Z","commit_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
  {"id":20,"user":{"login":"root","id":20},"body":"approved","state":"APPROVED","submitted_at":"2026-01-01T00:01:00Z","commit_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
  {"id":30,"user":{"login":"reply-review","id":30},"body":"","state":"COMMENTED","submitted_at":"2026-01-01T00:02:00Z","commit_id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
  {"id":50,"user":{"login":"dismissed","id":50},"body":"was approved","state":"DISMISSED","submitted_at":"2026-01-01T00:03:00Z","commit_id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
]`
		case "/api/graphql":
			graphqlRequests++
			var request struct {
				Query string `json:"query"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || !strings.Contains(request.Query, "createdAt") || strings.Contains(request.Query, "reviewThreads") {
				http.Error(w, "invalid reaction-only query", http.StatusBadRequest)
				return
			}
			body = `{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[{"databaseId":20,"reactions":{"nodes":[{"content":"HEART","createdAt":"2026-01-01T00:04:00Z","user":{"login":"fan","databaseId":60}}],"pageInfo":{"hasNextPage":false}}}],"pageInfo":{"hasNextPage":false}}}}}}`
		case "/api/v3/repos/owner/repo/pulls/7/comments":
			body = `[
  {"id":101,"pull_request_review_id":30,"diff_hunk":"","path":"","position":null,"commit_id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","user":{"login":"reply","id":30},"body":"reply","created_at":"2026-01-01T00:02:00Z","updated_at":"2026-01-01T00:02:00Z","in_reply_to_id":100},
  {"id":100,"pull_request_review_id":20,"diff_hunk":"@@ -3,2 +3,3 @@\n old\n+four\n+five","path":"a.go","position":2,"commit_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","user":{"login":"root","id":20},"body":"root","created_at":"2026-01-01T00:01:00Z","updated_at":"2026-01-01T00:01:00Z","line":5,"side":"RIGHT"},
  {"id":200,"pull_request_review_id":99,"diff_hunk":"@@ -1 +1 @@\n-old\n+new","path":"b.go","position":1,"commit_id":"cccccccccccccccccccccccccccccccccccccccc","user":{"login":"orphan","id":99},"body":"orphan","created_at":"2026-01-01T00:05:00Z","updated_at":"2026-01-01T00:05:00Z","line":1,"side":"RIGHT"}
]`
		case "/api/v3/repos/owner/repo/pulls/comments/100/reactions":
			if r.URL.Query().Get("page") == "1" {
				body = `[{"id":1,"user":{"login":"fan","id":60},"content":"+1","created_at":"2026-01-01T00:05:00Z"}]`
			} else {
				body = `[]`
			}
		case "/api/v3/repos/owner/repo/pulls/comments/101/reactions":
			if r.URL.Query().Get("page") == "1" {
				body = `[{"id":2,"user":{"login":"root","id":20},"content":"rocket","created_at":"2026-01-01T00:06:00Z"}]`
			} else {
				body = `[]`
			}
		case "/api/v3/repos/owner/repo/pulls/comments/200/reactions":
			body = `[]`
		default:
			http.Error(w, "unexpected request: "+r.URL.String(), http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	downloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", "", "owner", "repo")
	require.NoError(t, err)
	reviews, err := downloader.GetReviews(t.Context(), &base.PullRequest{
		Number:       7,
		ForeignIndex: 7,
		Context:      githubPullRequestContext{requestedReviewers: []*base.ExternalUser{{ID: 80, Name: "requested"}}},
	})
	require.NoError(t, err)
	require.Len(t, reviews, 5)
	assert.Equal(t, 1, graphqlRequests)

	assert.Equal(t, int64(10), reviews[0].ID)
	assert.Empty(t, reviews[0].Comments)
	assert.Equal(t, int64(20), reviews[1].ID)
	require.Len(t, reviews[1].Reactions, 1)
	assert.Equal(t, "heart", reviews[1].Reactions[0].Content)
	assert.Equal(t, time.Date(2026, 1, 1, 0, 4, 0, 0, time.UTC), reviews[1].Reactions[0].Created)
	require.Len(t, reviews[1].Comments, 2)
	assert.Equal(t, int64(20), reviews[1].Comments[0].PosterID)
	assert.Equal(t, "root", reviews[1].Comments[0].PosterName)
	require.Len(t, reviews[1].Comments[0].Reactions, 1)
	assert.Equal(t, "+1", reviews[1].Comments[0].Reactions[0].Content)
	assert.Equal(t, time.Date(2026, 1, 1, 0, 5, 0, 0, time.UTC), reviews[1].Comments[0].Reactions[0].Created)
	assert.Equal(t, int64(30), reviews[1].Comments[1].PosterID)
	assert.Equal(t, reviews[1].Comments[0].TreePath, reviews[1].Comments[1].TreePath)
	assert.Equal(t, reviews[1].Comments[0].Line, reviews[1].Comments[1].Line)
	require.Len(t, reviews[1].Comments[1].Reactions, 1)
	assert.Equal(t, "rocket", reviews[1].Comments[1].Reactions[0].Content)
	assert.Equal(t, time.Date(2026, 1, 1, 0, 6, 0, 0, time.UTC), reviews[1].Comments[1].Reactions[0].Created)

	assert.Equal(t, int64(50), reviews[2].ID)
	assert.Equal(t, base.ReviewStateCommented, reviews[2].State)
	assert.Equal(t, int64(99), reviews[3].ID)
	require.Len(t, reviews[3].Comments, 1)
	assert.Equal(t, "orphan", reviews[3].Comments[0].PosterName)
	assert.Equal(t, base.ReviewStateRequestReview, reviews[4].State)
	assert.Equal(t, int64(80), reviews[4].ReviewerID)
	assert.Equal(t, "requested", reviews[4].ReviewerName)
}

func TestGithubReviewReactionPagination(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var request struct {
			Query     string `json:"query"`
			Variables struct {
				ReviewCursor *string `json:"reviewCursor"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || strings.Contains(request.Query, "reviewThreads") {
			http.Error(w, "invalid review reaction request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			if request.Variables.ReviewCursor != nil {
				http.Error(w, "invalid initial cursor", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[{"databaseId":20,"reactions":{"nodes":[{"content":"HEART","createdAt":"2026-01-01T00:00:00Z","user":{"login":"fan","databaseId":60}}],"pageInfo":{"hasNextPage":false}}}],"pageInfo":{"hasNextPage":true,"endCursor":"next"}}}}}}`))
			return
		}
		if request.Variables.ReviewCursor == nil || *request.Variables.ReviewCursor != "next" {
			http.Error(w, "invalid second cursor", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[{"databaseId":30,"reactions":{"nodes":[{"content":"ROCKET","createdAt":"2026-01-01T00:01:00Z","user":{"login":"fan","databaseId":60}}],"pageInfo":{"hasNextPage":false}}}],"pageInfo":{"hasNextPage":false}}}}}}`))
	}))
	t.Cleanup(server.Close)

	downloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", "", "owner", "repo")
	require.NoError(t, err)
	reactions := downloader.reviewReactions(t.Context(), 7)
	assert.Equal(t, 2, requests)
	assert.Equal(t, "heart", reactions[20][0].Content)
	assert.Equal(t, "rocket", reactions[30][0].Content)
}

func TestGithubMultiToken(t *testing.T) {
	testCases := []struct {
		desc             string
		token            string
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
	}
	factory := GithubDownloaderV3Factory{}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			opts := base.MigrateOptions{CloneAddr: "https://github.com/go-gitea/gitea", AuthToken: tC.token}
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
