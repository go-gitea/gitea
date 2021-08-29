// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/migrations/base"

	"github.com/stretchr/testify/assert"
)

func TestGitlabDownloadRepo(t *testing.T) {
	// Skip tests if Gitlab token is not found
	gitlabPersonalAccessToken := os.Getenv("GITLAB_READ_TOKEN")
	if gitlabPersonalAccessToken == "" {
		t.Skip("skipped test because GITLAB_READ_TOKEN was not in the environment")
	}

	resp, err := http.Get("https://gitlab.com/gitea/test_repo")
	if err != nil || resp.StatusCode != 200 {
		t.Skipf("Can't access test repo, skipping %s", t.Name())
	}

	downloader, err := NewGitlabDownloader(context.Background(), "https://gitlab.com", "gitea/test_repo", "", "", gitlabPersonalAccessToken)
	if err != nil {
		t.Fatal(fmt.Sprintf("NewGitlabDownloader is nil: %v", err))
	}
	repo, err := downloader.GetRepoInfo()
	assert.NoError(t, err)
	// Repo Owner is blank in Gitlab Group repos
	assertRepositoryEqual(t, &base.Repository{
		Name:          "test_repo",
		Owner:         "",
		Description:   "Test repository for testing migration from gitlab to gitea",
		CloneURL:      "https://gitlab.com/gitea/test_repo.git",
		OriginalURL:   "https://gitlab.com/gitea/test_repo",
		DefaultBranch: "master",
	}, repo)

	topics, err := downloader.GetTopics()
	assert.NoError(t, err)
	assert.True(t, len(topics) == 2)
	assert.EqualValues(t, []string{"migration", "test"}, topics)

	milestones, err := downloader.GetMilestones()
	assert.NoError(t, err)
	assertMilestonesEqual(t, []*base.Milestone{
		{
			Title:   "1.1.0",
			Created: time.Date(2019, 11, 28, 8, 42, 44, 575000000, time.UTC),
			Updated: timePtr(time.Date(2019, 11, 28, 8, 42, 44, 575000000, time.UTC)),
			State:   "active",
		},
		{
			Title:   "1.0.0",
			Created: time.Date(2019, 11, 28, 8, 42, 30, 301000000, time.UTC),
			Updated: timePtr(time.Date(2019, 11, 28, 15, 57, 52, 401000000, time.UTC)),
			Closed:  timePtr(time.Date(2019, 11, 28, 15, 57, 52, 401000000, time.UTC)),
			State:   "closed",
		},
	}, milestones)

	labels, err := downloader.GetLabels()
	assert.NoError(t, err)
	assertLabelsEqual(t, []*base.Label{
		{
			Name:  "bug",
			Color: "d9534f",
		},
		{
			Name:  "confirmed",
			Color: "d9534f",
		},
		{
			Name:  "critical",
			Color: "d9534f",
		},
		{
			Name:  "discussion",
			Color: "428bca",
		},
		{
			Name:  "documentation",
			Color: "f0ad4e",
		},
		{
			Name:  "duplicate",
			Color: "7f8c8d",
		},
		{
			Name:  "enhancement",
			Color: "5cb85c",
		},
		{
			Name:  "suggestion",
			Color: "428bca",
		},
		{
			Name:  "support",
			Color: "f0ad4e",
		},
	}, labels)

	releases, err := downloader.GetReleases()
	assert.NoError(t, err)
	assertReleasesEqual(t, []*base.Release{
		{
			TagName:         "v0.9.99",
			TargetCommitish: "0720a3ec57c1f843568298117b874319e7deee75",
			Name:            "First Release",
			Body:            "A test release",
			Created:         time.Date(2019, 11, 28, 9, 9, 48, 840000000, time.UTC),
			PublisherID:     1241334,
			PublisherName:   "lafriks",
		},
	}, releases)

	issues, isEnd, err := downloader.GetIssues(1, 2)
	assert.NoError(t, err)
	assert.False(t, isEnd)

	assertIssuesEqual(t, []*base.Issue{
		{
			Number:     1,
			Title:      "Please add an animated gif icon to the merge button",
			Content:    "I just want the merge button to hurt my eyes a little. :stuck_out_tongue_closed_eyes:",
			Milestone:  "1.0.0",
			PosterID:   1241334,
			PosterName: "lafriks",
			State:      "closed",
			Created:    time.Date(2019, 11, 28, 8, 43, 35, 459000000, time.UTC),
			Updated:    time.Date(2019, 11, 28, 8, 46, 23, 304000000, time.UTC),
			Labels: []*base.Label{
				{
					Name: "bug",
				},
				{
					Name: "discussion",
				},
			},
			Reactions: []*base.Reaction{
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "thumbsup",
				},
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "open_mouth",
				}},
			Closed: timePtr(time.Date(2019, 11, 28, 8, 46, 23, 275000000, time.UTC)),
		},
		{
			Number:     2,
			Title:      "Test issue",
			Content:    "This is test issue 2, do not touch!",
			Milestone:  "1.1.0",
			PosterID:   1241334,
			PosterName: "lafriks",
			State:      "closed",
			Created:    time.Date(2019, 11, 28, 8, 44, 46, 277000000, time.UTC),
			Updated:    time.Date(2019, 11, 28, 8, 45, 44, 987000000, time.UTC),
			Labels: []*base.Label{
				{
					Name: "duplicate",
				},
			},
			Reactions: []*base.Reaction{
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "thumbsup",
				},
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "thumbsdown",
				},
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "laughing",
				},
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "tada",
				},
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "confused",
				},
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "hearts",
				}},
			Closed: timePtr(time.Date(2019, 11, 28, 8, 45, 44, 959000000, time.UTC)),
		},
	}, issues)

	comments, _, err := downloader.GetComments(base.GetCommentOptions{
		Context: gitlabIssueContext{
			foreignID:      2,
			localID:        2,
			IsMergeRequest: false,
		},
	})
	assert.NoError(t, err)
	assertCommentsEqual(t, []*base.Comment{
		{
			IssueIndex: 2,
			PosterID:   1241334,
			PosterName: "lafriks",
			Created:    time.Date(2019, 11, 28, 8, 44, 52, 501000000, time.UTC),
			Content:    "This is a comment",
			Reactions:  nil,
		},
		{
			IssueIndex: 2,
			PosterID:   1241334,
			PosterName: "lafriks",
			Created:    time.Date(2019, 11, 28, 8, 45, 2, 329000000, time.UTC),
			Content:    "changed milestone to %2",
			Reactions:  nil,
		},
		{
			IssueIndex: 2,
			PosterID:   1241334,
			PosterName: "lafriks",
			Created:    time.Date(2019, 11, 28, 8, 45, 45, 7000000, time.UTC),
			Content:    "closed",
			Reactions:  nil,
		},
		{
			IssueIndex: 2,
			PosterID:   1241334,
			PosterName: "lafriks",
			Created:    time.Date(2019, 11, 28, 8, 45, 53, 501000000, time.UTC),
			Content:    "A second comment",
			Reactions:  nil,
		},
	}, comments)

	prs, _, err := downloader.GetPullRequests(1, 1)
	assert.NoError(t, err)
	assertPullRequestsEqual(t, []*base.PullRequest{
		{
			Number:     4,
			Title:      "Test branch",
			Content:    "do not merge this PR",
			Milestone:  "1.0.0",
			PosterID:   1241334,
			PosterName: "lafriks",
			State:      "opened",
			Created:    time.Date(2019, 11, 28, 15, 56, 54, 104000000, time.UTC),
			Labels: []*base.Label{
				{
					Name: "bug",
				},
			},
			Reactions: []*base.Reaction{{
				UserID:   4575606,
				UserName: "real6543",
				Content:  "thumbsup",
			}, {
				UserID:   4575606,
				UserName: "real6543",
				Content:  "tada",
			}},
			PatchURL: "https://gitlab.com/gitea/test_repo/-/merge_requests/2.patch",
			Head: base.PullRequestBranch{
				Ref:       "feat/test",
				CloneURL:  "https://gitlab.com/gitea/test_repo/-/merge_requests/2",
				SHA:       "9f733b96b98a4175276edf6a2e1231489c3bdd23",
				RepoName:  "test_repo",
				OwnerName: "lafriks",
			},
			Base: base.PullRequestBranch{
				Ref:       "master",
				SHA:       "",
				OwnerName: "lafriks",
				RepoName:  "test_repo",
			},
			Closed:         nil,
			Merged:         false,
			MergedTime:     nil,
			MergeCommitSHA: "",
			Context: gitlabIssueContext{
				foreignID:      2,
				localID:        4,
				IsMergeRequest: true,
			},
		},
	}, prs)

	rvs, err := downloader.GetReviews(base.BasicIssueContext(1))
	assert.NoError(t, err)
	assertReviewsEqual(t, []*base.Review{
		{
			ReviewerID:   4102996,
			ReviewerName: "zeripath",
			CreatedAt:    time.Date(2019, 11, 28, 16, 02, 8, 377000000, time.UTC),
			State:        "APPROVED",
		},
		{
			ReviewerID:   527793,
			ReviewerName: "axifive",
			CreatedAt:    time.Date(2019, 11, 28, 16, 02, 8, 377000000, time.UTC),
			State:        "APPROVED",
		},
	}, rvs)

	rvs, err = downloader.GetReviews(base.BasicIssueContext(2))
	assert.NoError(t, err)
	assertReviewsEqual(t, []*base.Review{
		{
			ReviewerID:   4575606,
			ReviewerName: "real6543",
			CreatedAt:    time.Date(2020, 04, 19, 19, 24, 21, 108000000, time.UTC),
			State:        "APPROVED",
		},
	}, rvs)
}
