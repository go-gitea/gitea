// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	base "code.gitea.io/gitea/modules/migration"

	"github.com/stretchr/testify/assert"
)

func TestOneDevDownloadRepo(t *testing.T) {
	resp, err := http.Get("https://code.onedev.io/projects/go-gitea-test_repo")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skipf("Can't access test repo, skipping %s", t.Name())
	}

	u, _ := url.Parse("https://code.onedev.io")
	ctx := t.Context()
	downloader := NewOneDevDownloader(ctx, u, "", "", "go-gitea-test_repo")
	if err != nil {
		t.Fatalf("NewOneDevDownloader is nil: %v", err)
	}
	repo, err := downloader.GetRepoInfo(ctx)
	assert.NoError(t, err)
	assertRepositoryEqual(t, &base.Repository{
		Name:        "go-gitea-test_repo",
		Owner:       "",
		Description: "Test repository for testing migration from OneDev to gitea",
		CloneURL:    "https://code.onedev.io/go-gitea-test_repo",
		OriginalURL: "https://code.onedev.io/projects/go-gitea-test_repo",
	}, repo)

	milestones, err := downloader.GetMilestones(ctx)
	assert.NoError(t, err)
	deadline := time.Unix(1620086400, 0)
	assertMilestonesEqual(t, []*base.Milestone{
		{
			Title:    "1.0.0",
			Deadline: &deadline,
			Closed:   &deadline,
		},
		{
			Title:       "1.1.0",
			Description: "next things?",
		},
	}, milestones)

	labels, err := downloader.GetLabels(ctx)
	assert.NoError(t, err)
	assert.Len(t, labels, 6)

	issues, isEnd, err := downloader.GetIssues(ctx, 1, 2)
	assert.NoError(t, err)
	assert.False(t, isEnd)
	assertIssuesEqual(t, []*base.Issue{
		{
			Number:     4,
			Title:      "Hi there",
			Content:    "an issue not assigned to a milestone",
			PosterName: "User 336",
			State:      "open",
			Created:    time.Unix(1628549776, 734000000),
			Updated:    time.Unix(1628549776, 734000000),
			Labels: []*base.Label{
				{
					Name: "Improvement",
				},
			},
			ForeignIndex: 398,
			Context:      onedevIssueContext{IsPullRequest: false},
		},
		{
			Number:     3,
			Title:      "Add an awesome feature",
			Content:    "just another issue to test against",
			PosterName: "User 336",
			State:      "open",
			Milestone:  "1.1.0",
			Created:    time.Unix(1628549749, 878000000),
			Updated:    time.Unix(1628549749, 878000000),
			Labels: []*base.Label{
				{
					Name: "New Feature",
				},
			},
			ForeignIndex: 397,
			Context:      onedevIssueContext{IsPullRequest: false},
		},
	}, issues)

	comments, _, err := downloader.GetComments(ctx, &base.Issue{
		Number:       4,
		ForeignIndex: 398,
		Context:      onedevIssueContext{IsPullRequest: false},
	})
	assert.NoError(t, err)
	assertCommentsEqual(t, []*base.Comment{
		{
			IssueIndex: 4,
			PosterName: "User 336",
			Created:    time.Unix(1628549791, 128000000),
			Updated:    time.Unix(1628549791, 128000000),
			Content:    "it has a comment\n\nEDIT: that got edited",
		},
	}, comments)

	prs, _, err := downloader.GetPullRequests(ctx, 1, 1)
	assert.NoError(t, err)
	assertPullRequestsEqual(t, []*base.PullRequest{
		{
			Number:     5,
			Title:      "Pull to add a new file",
			Content:    "just do some git stuff",
			PosterName: "User 336",
			State:      "open",
			Created:    time.Unix(1628550076, 25000000),
			Updated:    time.Unix(1628550076, 25000000),
			Head: base.PullRequestBranch{
				Ref:      "branch-for-a-pull",
				SHA:      "343deffe3526b9bc84e873743ff7f6e6d8b827c0",
				RepoName: "go-gitea-test_repo",
			},
			Base: base.PullRequestBranch{
				Ref:      "master",
				SHA:      "f32b0a9dfd09a60f616f29158f772cedd89942d2",
				RepoName: "go-gitea-test_repo",
			},
			ForeignIndex: 186,
			Context:      onedevIssueContext{IsPullRequest: true},
		},
	}, prs)

	rvs, err := downloader.GetReviews(ctx, &base.PullRequest{Number: 5, ForeignIndex: 186})
	assert.NoError(t, err)
	assertReviewsEqual(t, []*base.Review{
		{
			IssueIndex:   5,
			ReviewerName: "User 317",
			State:        "PENDING",
		},
	}, rvs)
}
