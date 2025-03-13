// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"os"
	"testing"
	"time"

	base "code.gitea.io/gitea/modules/migration"

	"github.com/stretchr/testify/assert"
)

func TestGitHubDownloadRepo(t *testing.T) {
	GithubLimitRateRemaining = 3 // Wait at 3 remaining since we could have 3 CI in //
	token := os.Getenv("GITHUB_READ_TOKEN")
	if token == "" {
		t.Skip("Skipping GitHub migration test because GITHUB_READ_TOKEN is empty")
	}
	ctx := t.Context()
	downloader := NewGithubDownloaderV3(ctx, "https://github.com", "", "", token, "go-gitea", "test_repo")
	err := downloader.RefreshRate(ctx)
	assert.NoError(t, err)

	repo, err := downloader.GetRepoInfo(ctx)
	assert.NoError(t, err)
	assertRepositoryEqual(t, &base.Repository{
		Name:          "test_repo",
		Owner:         "go-gitea",
		Description:   "Test repository for testing migration from github to gitea",
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
			Deadline:    timePtr(time.Date(2019, 11, 11, 8, 0, 0, 0, time.UTC)),
			Created:     time.Date(2019, 11, 12, 19, 37, 8, 0, time.UTC),
			Updated:     timePtr(time.Date(2019, 11, 12, 21, 56, 17, 0, time.UTC)),
			Closed:      timePtr(time.Date(2019, 11, 12, 19, 45, 49, 0, time.UTC)),
			State:       "closed",
		},
		{
			Title:       "1.1.0",
			Description: "Milestone 1.1.0",
			Deadline:    timePtr(time.Date(2019, 11, 12, 8, 0, 0, 0, time.UTC)),
			Created:     time.Date(2019, 11, 12, 19, 37, 25, 0, time.UTC),
			Updated:     timePtr(time.Date(2019, 11, 12, 21, 39, 27, 0, time.UTC)),
			Closed:      timePtr(time.Date(2019, 11, 12, 19, 45, 46, 0, time.UTC)),
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
			Closed: timePtr(time.Date(2019, 11, 12, 20, 22, 22, 0, time.UTC)),
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
			Closed: timePtr(time.Date(2019, 11, 12, 21, 1, 31, 0, time.UTC)),
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
			PatchURL: "https://github.com/go-gitea/test_repo/pull/3.patch",
			Head: base.PullRequestBranch{
				Ref:      "master",
				CloneURL: "https://github.com/mrsdizzie/test_repo.git",
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
			Closed:         timePtr(time.Date(2019, 11, 12, 21, 39, 27, 0, time.UTC)),
			Merged:         true,
			MergedTime:     timePtr(time.Date(2019, 11, 12, 21, 39, 27, 0, time.UTC)),
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
			Updated:    time.Date(2020, 1, 4, 11, 30, 1, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:        "bug",
					Color:       "d73a4a",
					Description: "Something isn't working",
				},
			},
			PatchURL: "https://github.com/go-gitea/test_repo/pull/4.patch",
			Head: base.PullRequestBranch{
				Ref:       "test-branch",
				SHA:       "2be9101c543658591222acbee3eb799edfc3853d",
				RepoName:  "test_repo",
				OwnerName: "mrsdizzie",
				CloneURL:  "https://github.com/mrsdizzie/test_repo.git",
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
