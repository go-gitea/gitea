// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	base "code.gitea.io/gitea/modules/migration"

	"github.com/stretchr/testify/assert"
)

func TestCodebaseDownloadRepo(t *testing.T) {
	// Skip tests if Codebase token is not found
	cloneUser := os.Getenv("CODEBASE_CLONE_USER")
	clonePassword := os.Getenv("CODEBASE_CLONE_PASSWORD")
	apiUser := os.Getenv("CODEBASE_API_USER")
	apiPassword := os.Getenv("CODEBASE_API_TOKEN")
	if apiUser == "" || apiPassword == "" {
		t.Skip("skipped test because a CODEBASE_ variable was not in the environment")
	}

	cloneAddr := "https://gitea-test.codebasehq.com/gitea-test/test.git"
	u, _ := url.Parse(cloneAddr)
	if cloneUser != "" {
		u.User = url.UserPassword(cloneUser, clonePassword)
	}

	factory := &CodebaseDownloaderFactory{}
	downloader, err := factory.New(context.Background(), base.MigrateOptions{
		CloneAddr:    u.String(),
		AuthUsername: apiUser,
		AuthPassword: apiPassword,
	})
	if err != nil {
		t.Fatal(fmt.Sprintf("Error creating Codebase downloader: %v", err))
	}
	repo, err := downloader.GetRepoInfo()
	assert.NoError(t, err)
	assertRepositoryEqual(t, &base.Repository{
		Name:        "test",
		Owner:       "",
		Description: "Repository Description",
		CloneURL:    "git@codebasehq.com:gitea-test/gitea-test/test.git",
		OriginalURL: cloneAddr,
	}, repo)

	milestones, err := downloader.GetMilestones()
	assert.NoError(t, err)
	assertMilestonesEqual(t, []*base.Milestone{
		{
			Title:    "Milestone1",
			Deadline: timePtr(time.Date(2021, time.September, 16, 0, 0, 0, 0, time.UTC)),
		},
		{
			Title:    "Milestone2",
			Deadline: timePtr(time.Date(2021, time.September, 17, 0, 0, 0, 0, time.UTC)),
			Closed:   timePtr(time.Date(2021, time.September, 17, 0, 0, 0, 0, time.UTC)),
			State:    "closed",
		},
	}, milestones)

	labels, err := downloader.GetLabels()
	assert.NoError(t, err)
	assert.Len(t, labels, 4)

	issues, isEnd, err := downloader.GetIssues(1, 2)
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assertIssuesEqual(t, []*base.Issue{
		{
			Number:      2,
			Title:       "Open Ticket",
			Content:     "Open Ticket Message",
			PosterName:  "gitea-test-43",
			PosterEmail: "gitea-codebase@smack.email",
			State:       "open",
			Created:     time.Date(2021, time.September, 26, 19, 19, 14, 0, time.UTC),
			Updated:     time.Date(2021, time.September, 26, 19, 19, 34, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name: "Feature",
				},
			},
		},
		{
			Number:      1,
			Title:       "Closed Ticket",
			Content:     "Closed Ticket Message",
			PosterName:  "gitea-test-43",
			PosterEmail: "gitea-codebase@smack.email",
			State:       "closed",
			Milestone:   "Milestone1",
			Created:     time.Date(2021, time.September, 26, 19, 18, 33, 0, time.UTC),
			Updated:     time.Date(2021, time.September, 26, 19, 18, 55, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name: "Bug",
				},
			},
		},
	}, issues)

	comments, _, err := downloader.GetComments(base.GetCommentOptions{
		Context: issues[0].Context,
	})
	assert.NoError(t, err)
	assertCommentsEqual(t, []*base.Comment{
		{
			IssueIndex:  2,
			PosterName:  "gitea-test-43",
			PosterEmail: "gitea-codebase@smack.email",
			Created:     time.Date(2021, time.September, 26, 19, 19, 34, 0, time.UTC),
			Updated:     time.Date(2021, time.September, 26, 19, 19, 34, 0, time.UTC),
			Content:     "open comment",
		},
	}, comments)

	prs, _, err := downloader.GetPullRequests(1, 1)
	assert.NoError(t, err)
	assertPullRequestsEqual(t, []*base.PullRequest{
		{
			Number:      3,
			Title:       "Readme Change",
			Content:     "Merge Request comment",
			PosterName:  "gitea-test-43",
			PosterEmail: "gitea-codebase@smack.email",
			State:       "open",
			Created:     time.Date(2021, time.September, 26, 20, 25, 47, 0, time.UTC),
			Updated:     time.Date(2021, time.September, 26, 20, 25, 47, 0, time.UTC),
			Head: base.PullRequestBranch{
				Ref:      "readme-mr",
				SHA:      "1287f206b888d4d13540e0a8e1c07458f5420059",
				RepoName: "test",
			},
			Base: base.PullRequestBranch{
				Ref:      "master",
				SHA:      "f32b0a9dfd09a60f616f29158f772cedd89942d2",
				RepoName: "test",
			},
		},
	}, prs)

	rvs, err := downloader.GetReviews(prs[0].Context)
	assert.NoError(t, err)
	assert.Empty(t, rvs)
}
