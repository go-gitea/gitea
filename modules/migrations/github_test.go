// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/migrations/base"

	"github.com/stretchr/testify/assert"
)

func assertMilestoneEqual(t *testing.T, description, title, dueOn, created, updated, closed, state string, ms *base.Milestone) {
	var tmPtr *time.Time
	if dueOn != "" {
		tm, err := time.Parse("2006-01-02 15:04:05 -0700 MST", dueOn)
		assert.NoError(t, err)
		tmPtr = &tm
	}
	var (
		createdTM time.Time
		updatedTM *time.Time
		closedTM  *time.Time
	)
	if created != "" {
		var err error
		createdTM, err = time.Parse("2006-01-02 15:04:05 -0700 MST", created)
		assert.NoError(t, err)
	}
	if updated != "" {
		updatedTemp, err := time.Parse("2006-01-02 15:04:05 -0700 MST", updated)
		assert.NoError(t, err)
		updatedTM = &updatedTemp
	}
	if closed != "" {
		closedTemp, err := time.Parse("2006-01-02 15:04:05 -0700 MST", closed)
		assert.NoError(t, err)
		closedTM = &closedTemp
	}

	assert.EqualValues(t, &base.Milestone{
		Description: description,
		Title:       title,
		Deadline:    tmPtr,
		State:       state,
		Created:     createdTM,
		Updated:     updatedTM,
		Closed:      closedTM,
	}, ms)
}

func assertLabelEqual(t *testing.T, name, color, description string, label *base.Label) {
	assert.EqualValues(t, &base.Label{
		Name:        name,
		Color:       color,
		Description: description,
	}, label)
}

func TestGitHubDownloadRepo(t *testing.T) {
	GithubLimitRateRemaining = 3 //Wait at 3 remaining since we could have 3 CI in //
	downloader := NewGithubDownloaderV3(os.Getenv("GITHUB_READ_TOKEN"), "", "go-gitea", "test_repo")
	err := downloader.RefreshRate()
	assert.NoError(t, err)

	repo, err := downloader.GetRepoInfo()
	assert.NoError(t, err)
	assert.EqualValues(t, &base.Repository{
		Name:        "test_repo",
		Owner:       "go-gitea",
		Description: "Test repository for testing migration from github to gitea",
		CloneURL:    "https://github.com/go-gitea/test_repo.git",
		OriginalURL: "https://github.com/go-gitea/test_repo",
	}, repo)

	topics, err := downloader.GetTopics()
	assert.NoError(t, err)
	assert.Contains(t, topics, "gitea")

	milestones, err := downloader.GetMilestones()
	assert.NoError(t, err)
	assert.True(t, len(milestones) >= 2)

	for _, milestone := range milestones {
		switch milestone.Title {
		case "1.0.0":
			assertMilestoneEqual(t, "Milestone 1.0.0", "1.0.0", "2019-11-11 08:00:00 +0000 UTC",
				"2019-11-12 19:37:08 +0000 UTC",
				"2019-11-12 21:56:17 +0000 UTC",
				"2019-11-12 19:45:49 +0000 UTC",
				"closed", milestone)
		case "1.1.0":
			assertMilestoneEqual(t, "Milestone 1.1.0", "1.1.0", "2019-11-12 08:00:00 +0000 UTC",
				"2019-11-12 19:37:25 +0000 UTC",
				"2019-11-12 21:39:27 +0000 UTC",
				"2019-11-12 19:45:46 +0000 UTC",
				"closed", milestone)
		}
	}

	labels, err := downloader.GetLabels()
	assert.NoError(t, err)
	assert.True(t, len(labels) >= 8)
	for _, l := range labels {
		switch l.Name {
		case "bug":
			assertLabelEqual(t, "bug", "d73a4a", "Something isn't working", l)
		case "documentation":
			assertLabelEqual(t, "documentation", "0075ca", "Improvements or additions to documentation", l)
		case "duplicate":
			assertLabelEqual(t, "duplicate", "cfd3d7", "This issue or pull request already exists", l)
		case "enhancement":
			assertLabelEqual(t, "enhancement", "a2eeef", "New feature or request", l)
		case "good first issue":
			assertLabelEqual(t, "good first issue", "7057ff", "Good for newcomers", l)
		case "help wanted":
			assertLabelEqual(t, "help wanted", "008672", "Extra attention is needed", l)
		case "invalid":
			assertLabelEqual(t, "invalid", "e4e669", "This doesn't seem right", l)
		case "question":
			assertLabelEqual(t, "question", "d876e3", "Further information is requested", l)
		}
	}

	releases, err := downloader.GetReleases()
	assert.NoError(t, err)
	assert.EqualValues(t, []*base.Release{
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
	}, releases[len(releases)-1:])

	// downloader.GetIssues()
	issues, isEnd, err := downloader.GetIssues(1, 2)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(issues))
	assert.False(t, isEnd)

	var (
		closed1 = time.Date(2019, 11, 12, 20, 22, 22, 0, time.UTC)
		closed2 = time.Date(2019, 11, 12, 21, 1, 31, 0, time.UTC)
	)
	assert.EqualValues(t, []*base.Issue{
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
			Reactions: &base.Reactions{
				TotalCount: 1,
				PlusOne:    1,
				MinusOne:   0,
				Laugh:      0,
				Confused:   0,
				Heart:      0,
				Hooray:     0,
			},
			Closed: &closed1,
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
			Reactions: &base.Reactions{
				TotalCount: 6,
				PlusOne:    1,
				MinusOne:   1,
				Laugh:      1,
				Confused:   1,
				Heart:      1,
				Hooray:     1,
			},
			Closed: &closed2,
		},
	}, issues)

	// downloader.GetComments()
	comments, err := downloader.GetComments(2)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(comments))
	assert.EqualValues(t, []*base.Comment{
		{
			IssueIndex: 2,
			PosterID:   1669571,
			PosterName: "mrsdizzie",
			Created:    time.Date(2019, 11, 12, 21, 0, 13, 0, time.UTC),
			Updated:    time.Date(2019, 11, 12, 21, 0, 13, 0, time.UTC),
			Content:    "This is a comment",
			Reactions: &base.Reactions{
				TotalCount: 1,
				PlusOne:    1,
				MinusOne:   0,
				Laugh:      0,
				Confused:   0,
				Heart:      0,
				Hooray:     0,
			},
		},
		{
			IssueIndex: 2,
			PosterID:   1669571,
			PosterName: "mrsdizzie",
			Created:    time.Date(2019, 11, 12, 22, 7, 14, 0, time.UTC),
			Updated:    time.Date(2019, 11, 12, 22, 7, 14, 0, time.UTC),
			Content:    "A second comment",
			Reactions: &base.Reactions{
				TotalCount: 0,
				PlusOne:    0,
				MinusOne:   0,
				Laugh:      0,
				Confused:   0,
				Heart:      0,
				Hooray:     0,
			},
		},
	}, comments[:2])

	// downloader.GetPullRequests()
	prs, err := downloader.GetPullRequests(1, 2)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(prs))

	closed1 = time.Date(2019, 11, 12, 21, 39, 27, 0, time.UTC)
	var merged1 = time.Date(2019, 11, 12, 21, 39, 27, 0, time.UTC)

	assert.EqualValues(t, []*base.PullRequest{
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
			Closed:         &closed1,
			Merged:         true,
			MergedTime:     &merged1,
			MergeCommitSHA: "f32b0a9dfd09a60f616f29158f772cedd89942d2",
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
		},
	}, prs)
}
