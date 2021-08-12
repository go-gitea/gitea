// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/migrations/base"

	"github.com/stretchr/testify/assert"
)

func TestOneDevDownloadRepo(t *testing.T) {
	resp, err := http.Get("https://code.onedev.io/projects/go-gitea-test_repo")
	if err != nil || resp.StatusCode != 200 {
		t.Skipf("Can't access test repo, skipping %s", t.Name())
	}

	u, _ := url.Parse("https://code.onedev.io")
	downloader := NewOneDevDownloader(context.Background(), u, "", "", "go-gitea-test_repo")
	if err != nil {
		t.Fatal(fmt.Sprintf("NewOneDevDownloader is nil: %v", err))
	}
	repo, err := downloader.GetRepoInfo()
	assert.NoError(t, err)
	assert.EqualValues(t, &base.Repository{
		Name:        "go-gitea-test_repo",
		Owner:       "",
		Description: "Test repository for testing migration from OneDev to gitea",
		CloneURL:    "https://code.onedev.io/go-gitea-test_repo",
		OriginalURL: "https://code.onedev.io/projects/go-gitea-test_repo",
	}, repo)

	milestones, err := downloader.GetMilestones()
	assert.NoError(t, err)
	assert.Len(t, milestones, 2)
	assertMilestoneEqual(t, "", "1.0.0",
		"2021-05-04 00:00:00.000 +0000 UTC",
		"",
		"",
		"2021-05-04 00:00:00.000 +0000 UTC",
		"", milestones[0])
	assertMilestoneEqual(t, "next things?", "1.1.0",
		"",
		"",
		"",
		"",
		"", milestones[1])

	labels, err := downloader.GetLabels()
	assert.NoError(t, err)
	assert.Len(t, labels, 6)

	issues, isEnd, err := downloader.GetIssues(1, 2)
	assert.NoError(t, err)
	assert.Len(t, issues, 2)
	assert.False(t, isEnd)

	assert.EqualValues(t, []*base.Issue{
		{
			Number:     4,
			Title:      "Hi there",
			Content:    "an issue not assigned to a milestone",
			PosterName: "User 336",
			State:      "open",
			Created:    time.Date(2021, 8, 9, 22, 56, 16, 734000000, time.UTC),
			Updated:    time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name: "Improvement",
				},
			},
			Context: onedevIssueContext{
				foreignID:     398,
				localID:       4,
				IsPullRequest: false,
			},
		},
		{
			Number:     3,
			Title:      "Add an awesome feature",
			Content:    "just another issue to test against",
			PosterName: "User 336",
			State:      "open",
			Milestone:  "1.1.0",
			Created:    time.Date(2021, 8, 9, 22, 55, 49, 878000000, time.UTC),
			Updated:    time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name: "New Feature",
				},
			},
			Context: onedevIssueContext{
				foreignID:     397,
				localID:       3,
				IsPullRequest: false,
			},
		},
	}, issues)

	comments, _, err := downloader.GetComments(base.GetCommentOptions{
		Context: onedevIssueContext{
			foreignID:     398,
			localID:       4,
			IsPullRequest: false,
		},
	})
	assert.NoError(t, err)
	assert.Len(t, comments, 1)
	assert.EqualValues(t, []*base.Comment{
		{
			IssueIndex: 4,
			PosterName: "User 336",
			Created:    time.Date(2021, 8, 9, 22, 56, 31, 128000000, time.UTC),
			Updated:    time.Date(2021, 8, 9, 22, 56, 31, 128000000, time.UTC),
			Content:    "it has a comment\r\n\r\nEDIT: that got edited",
		},
	}, comments)

	prs, _, err := downloader.GetPullRequests(1, 1)
	assert.NoError(t, err)
	assert.Len(t, prs, 1)

	assert.EqualValues(t, []*base.PullRequest{
		{
			Number:     5,
			Title:      "Pull to add a new file",
			Content:    "just do some git stuff",
			PosterName: "User 336",
			State:      "open",
			Created:    time.Date(2021, 8, 9, 23, 1, 16, 025000000, time.UTC),
			Updated:    time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
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
			Context: onedevIssueContext{
				foreignID:     186,
				localID:       5,
				IsPullRequest: true,
			},
		},
	}, prs)

	rvs, err := downloader.GetReviews(onedevIssueContext{
		foreignID: 186,
		localID:   5,
	})
	assert.NoError(t, err)
	assert.Len(t, rvs, 1)
	assert.Equal(t, "User 317", rvs[0].ReviewerName)
	assert.Equal(t, "PENDING", rvs[0].State)
	assert.Empty(t, rvs[0].Content)
}
