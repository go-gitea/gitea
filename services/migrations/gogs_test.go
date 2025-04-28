// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"net/http"
	"os"
	"testing"
	"time"

	base "code.gitea.io/gitea/modules/migration"

	"github.com/stretchr/testify/assert"
)

func TestGogsDownloadRepo(t *testing.T) {
	// Skip tests if Gogs token is not found
	gogsPersonalAccessToken := os.Getenv("GOGS_READ_TOKEN")
	if len(gogsPersonalAccessToken) == 0 {
		t.Skip("skipped test because GOGS_READ_TOKEN was not in the environment")
	}

	resp, err := http.Get("https://try.gogs.io/lunnytest/TESTREPO")
	if err != nil || resp.StatusCode/100 != 2 {
		// skip and don't run test
		t.Skipf("visit test repo failed, ignored")
		return
	}
	ctx := t.Context()
	downloader := NewGogsDownloader(ctx, "https://try.gogs.io", "", "", gogsPersonalAccessToken, "lunnytest", "TESTREPO")
	repo, err := downloader.GetRepoInfo(ctx)
	assert.NoError(t, err)

	assertRepositoryEqual(t, &base.Repository{
		Name:          "TESTREPO",
		Owner:         "lunnytest",
		Description:   "",
		CloneURL:      "https://try.gogs.io/lunnytest/TESTREPO.git",
		OriginalURL:   "https://try.gogs.io/lunnytest/TESTREPO",
		DefaultBranch: "master",
	}, repo)

	milestones, err := downloader.GetMilestones(ctx)
	assert.NoError(t, err)
	assertMilestonesEqual(t, []*base.Milestone{
		{
			Title: "1.0",
			State: "open",
		},
	}, milestones)

	labels, err := downloader.GetLabels(ctx)
	assert.NoError(t, err)
	assertLabelsEqual(t, []*base.Label{
		{
			Name:  "bug",
			Color: "ee0701",
		},
		{
			Name:  "duplicate",
			Color: "cccccc",
		},
		{
			Name:  "enhancement",
			Color: "84b6eb",
		},
		{
			Name:  "help wanted",
			Color: "128a0c",
		},
		{
			Name:  "invalid",
			Color: "e6e6e6",
		},
		{
			Name:  "question",
			Color: "cc317c",
		},
		{
			Name:  "wontfix",
			Color: "ffffff",
		},
	}, labels)

	// downloader.GetIssues()
	issues, isEnd, err := downloader.GetIssues(ctx, 1, 8)
	assert.NoError(t, err)
	assert.False(t, isEnd)
	assertIssuesEqual(t, []*base.Issue{
		{
			Number:      1,
			PosterID:    5331,
			PosterName:  "lunny",
			PosterEmail: "xiaolunwen@gmail.com",
			Title:       "test",
			Content:     "test",
			Milestone:   "",
			State:       "open",
			Created:     time.Date(2019, 6, 11, 8, 16, 44, 0, time.UTC),
			Updated:     time.Date(2019, 10, 26, 11, 7, 2, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:  "bug",
					Color: "ee0701",
				},
			},
		},
	}, issues)

	// downloader.GetComments()
	comments, _, err := downloader.GetComments(ctx, &base.Issue{Number: 1, ForeignIndex: 1})
	assert.NoError(t, err)
	assertCommentsEqual(t, []*base.Comment{
		{
			IssueIndex:  1,
			PosterID:    5331,
			PosterName:  "lunny",
			PosterEmail: "xiaolunwen@gmail.com",
			Created:     time.Date(2019, 6, 11, 8, 19, 50, 0, time.UTC),
			Updated:     time.Date(2019, 6, 11, 8, 19, 50, 0, time.UTC),
			Content:     "1111",
		},
		{
			IssueIndex:  1,
			PosterID:    15822,
			PosterName:  "clacplouf",
			PosterEmail: "test1234@dbn.re",
			Created:     time.Date(2019, 10, 26, 11, 7, 2, 0, time.UTC),
			Updated:     time.Date(2019, 10, 26, 11, 7, 2, 0, time.UTC),
			Content:     "88888888",
		},
	}, comments)

	// downloader.GetPullRequests()
	_, _, err = downloader.GetPullRequests(ctx, 1, 3)
	assert.Error(t, err)
}
