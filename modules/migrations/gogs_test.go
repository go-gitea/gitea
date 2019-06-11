// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"testing"
	"time"

	"code.gitea.io/gitea/modules/migrations/base"
	"github.com/stretchr/testify/assert"
)

func TestGogsDownloadRepo(t *testing.T) {
	downloader := NewGogsDownloader("https://try.gogs.io", "c109b3c905eb57951cfdea270cfcfdc297a74500", "", "lunnytest", "TESTREPO")
	repo, err := downloader.GetRepoInfo()
	assert.NoError(t, err)
	assert.EqualValues(t, &base.Repository{
		Name:        "TESTREPO",
		Owner:       "lunnytest",
		Description: "",
		CloneURL:    "https://try.gogs.io/lunnytest/TESTREPO.git",
	}, repo)

	milestones, err := downloader.GetMilestones()
	assert.NoError(t, err)
	assert.True(t, len(milestones) == 1)

	for _, milestone := range milestones {
		switch milestone.Title {
		case "1.0":
			assert.EqualValues(t, "open", milestone.State)
		}
	}

	labels, err := downloader.GetLabels()
	assert.NoError(t, err)
	assert.True(t, len(labels) == 7)
	for _, l := range labels {
		switch l.Name {
		case "bug":
			assertLabelEqual(t, "bug", "ee0701", l)
		case "duplicated":
			assertLabelEqual(t, "duplicated", "cccccc", l)
		case "enhancement":
			assertLabelEqual(t, "enhancement", "84b6eb", l)
		case "help wanted":
			assertLabelEqual(t, "help wanted", "128a0c", l)
		case "invalid":
			assertLabelEqual(t, "invalid", "e6e6e6", l)
		case "question":
			assertLabelEqual(t, "question", "cc317c", l)
		case "wontfix":
			assertLabelEqual(t, "wontfix", "ffffff", l)
		}
	}

	_, err = downloader.GetReleases()
	assert.Error(t, err)

	// downloader.GetIssues()
	issues, isEnd, err := downloader.GetIssues(1, 8)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, len(issues))
	assert.False(t, isEnd)

	assert.EqualValues(t, []*base.Issue{
		{
			Number:      1,
			Title:       "test",
			Content:     "test",
			Milestone:   "",
			PosterName:  "lunny",
			PosterEmail: "xiaolunwen@gmail.com",
			State:       "open",
			Created:     time.Date(2019, 06, 11, 8, 16, 44, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:  "bug",
					Color: "ee0701",
				},
			},
		},
	}, issues)

	// downloader.GetComments()
	comments, err := downloader.GetComments(1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, len(comments))
	assert.EqualValues(t, []*base.Comment{
		{
			PosterName:  "lunny",
			PosterEmail: "xiaolunwen@gmail.com",
			Created:     time.Date(2019, 06, 11, 8, 19, 50, 0, time.UTC),
			Updated:     time.Date(2019, 06, 11, 8, 19, 50, 0, time.UTC),
			Content:     `1111`,
		},
	}, comments)

	// downloader.GetPullRequests()
	_, err = downloader.GetPullRequests(1, 3)
	assert.Error(t, err)
}
