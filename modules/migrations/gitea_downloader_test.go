// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/migrations/base"

	"github.com/stretchr/testify/assert"
)

func assertEqualIssue(t *testing.T, issueExp, IssueGet *base.Issue) {
	assert.EqualValues(t, issueExp.Number, IssueGet.Number)
	assert.EqualValues(t, issueExp.Title, IssueGet.Title)
	assert.EqualValues(t, issueExp.Content, IssueGet.Content)
	assert.EqualValues(t, issueExp.Milestone, IssueGet.Milestone)
	assert.EqualValues(t, issueExp.PosterID, IssueGet.PosterID)
	assert.EqualValues(t, issueExp.PosterName, IssueGet.PosterName)
	assert.EqualValues(t, issueExp.PosterEmail, IssueGet.PosterEmail)
	assert.EqualValues(t, issueExp.IsLocked, IssueGet.IsLocked)
	assert.EqualValues(t, issueExp.Created.Unix(), IssueGet.Created.Unix())
	assert.EqualValues(t, issueExp.Updated.Unix(), IssueGet.Updated.Unix())
	if issueExp.Closed != nil {
		assert.EqualValues(t, issueExp.Closed.Unix(), IssueGet.Closed.Unix())
	} else {
		assert.True(t, IssueGet.Closed == nil)
	}
	assert.EqualValues(t, issueExp.Labels, IssueGet.Labels)
	assert.EqualValues(t, issueExp.Reactions, IssueGet.Reactions)
}

func TestGiteaDownloadRepo(t *testing.T) {
	// Skip tests if Gitea token is not found
	giteaToken := os.Getenv("GITEA_TOKEN")
	if giteaToken == "" {
		t.Skip("skipped test because GITEA_TOKEN was not in the environment")
	}

	resp, err := http.Get("https://gitea.com/gitea")
	if err != nil || resp.StatusCode != 200 {
		t.Skipf("Can't reach https://gitea.com, skipping %s", t.Name())
	}

	downloader := NewGiteaDownloader("https://gitea.com", "gitea/test_repo", "", "", giteaToken)
	if downloader == nil {
		t.Fatal("NewGitlabDownloader is nil")
	}

	repo, err := downloader.GetRepoInfo()
	assert.NoError(t, err)
	assert.EqualValues(t, &base.Repository{
		Name:        "test_repo",
		Owner:       "gitea",
		IsPrivate:   false, // ToDo: set test repo private
		Description: "Test repository for testing migration from gitea to gitea",
		CloneURL:    "https://gitea.com/gitea/test_repo.git",
		OriginalURL: "https://gitea.com/gitea/test_repo",
	}, repo)

	topics, err := downloader.GetTopics()
	assert.NoError(t, err)
	sort.Strings(topics)
	assert.EqualValues(t, []string{"ci", "gitea", "migration", "test"}, topics)

	labels, err := downloader.GetLabels()
	assert.NoError(t, err)
	assert.Len(t, labels, 6)
	for _, l := range labels {
		switch l.Name {
		case "Bug":
			assertLabelEqual(t, "Bug", "e11d21", "", l)
		case "documentation":
			assertLabelEqual(t, "Enhancement", "207de5", "", l)
		case "confirmed":
			assertLabelEqual(t, "Feature", "0052cc", "a feature request", l)
		case "enhancement":
			assertLabelEqual(t, "Invalid", "d4c5f9", "", l)
		case "critical":
			assertLabelEqual(t, "Question", "fbca04", "", l)
		case "discussion":
			assertLabelEqual(t, "Valid", "53e917", "", l)
		default:
			assert.Error(t, fmt.Errorf("unexpected label: %s", l.Name))
		}
	}

	milestones, err := downloader.GetMilestones()
	assert.NoError(t, err)
	assert.Len(t, milestones, 2)

	for _, milestone := range milestones {
		switch milestone.Title {
		case "V1":
			assert.EqualValues(t, "Generate Content", milestone.Description)
			// assert.EqualValues(t, "ToDo", milestone.Created)
			// assert.EqualValues(t, "ToDo", milestone.Updated)
			assert.EqualValues(t, 1598985406, milestone.Closed.Unix())
			assert.True(t, milestone.Deadline == nil)
			assert.EqualValues(t, "closed", milestone.State)
		case "V2 Finalize":
			assert.EqualValues(t, "", milestone.Description)
			// assert.EqualValues(t, "ToDo", milestone.Created)
			// assert.EqualValues(t, "ToDo", milestone.Updated)
			assert.True(t, milestone.Closed == nil)
			assert.EqualValues(t, 1599263999, milestone.Deadline.Unix())
			assert.EqualValues(t, "open", milestone.State)
		default:
			assert.Error(t, fmt.Errorf("unexpected milestone: %s", milestone.Title))
		}
	}

	releases, err := downloader.GetReleases()
	assert.NoError(t, err)
	assert.EqualValues(t, []*base.Release{
		{
			Name:            "Second Release",
			TagName:         "v2-rc1",
			TargetCommitish: "master",
			Body:            "this repo has:\r\n* reactions\r\n* wiki\r\n* issues  (open/closed)\r\n* pulls (open/closed/merged) (external/internal)\r\n* pull reviews\r\n* projects\r\n* milestones\r\n* lables\r\n* releases\r\n\r\nto test migration agains",
			Draft:           false,
			Prerelease:      true,
			Created:         time.Date(2020, 9, 1, 18, 2, 43, 0, time.UTC),
			Published:       time.Date(2020, 9, 1, 18, 2, 43, 0, time.UTC),
			PublisherID:     689,
			PublisherName:   "6543",
			PublisherEmail:  "6543@noreply.gitea.io",
		},
		{
			Name:            "First Release",
			TagName:         "V1",
			TargetCommitish: "master",
			Body:            "as title",
			Draft:           false,
			Prerelease:      false,
			Created:         time.Date(2020, 9, 1, 17, 30, 32, 0, time.UTC),
			Published:       time.Date(2020, 9, 1, 17, 30, 32, 0, time.UTC),
			PublisherID:     689,
			PublisherName:   "6543",
			PublisherEmail:  "6543@noreply.gitea.io",
		},
	}, releases)

	issues, isEnd, err := downloader.GetIssues(1, 50)
	assert.NoError(t, err)
	assert.EqualValues(t, 7, len(issues))
	assert.True(t, isEnd)
	assert.EqualValues(t, "open", issues[0].State)

	issues, isEnd, err = downloader.GetIssues(3, 2)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(issues))
	assert.False(t, isEnd)

	var (
		closed4 = time.Date(2020, 9, 1, 15, 49, 34, 0, time.UTC)
		closed2 = time.Unix(1598969497, 0)
	)

	assertEqualIssue(t, &base.Issue{
		Number:      4,
		Title:       "what is this repo about?",
		Content:     "",
		Milestone:   "V1",
		PosterID:    -1,
		PosterName:  "Ghost",
		PosterEmail: "",
		State:       "closed",
		IsLocked:    true,
		Created:     time.Unix(1598975321, 0),
		Updated:     time.Unix(1598975400, 0),
		Labels: []*base.Label{{
			Name:        "Question",
			Color:       "fbca04",
			Description: "",
		}},
		Reactions: []*base.Reaction{
			{
				UserID:   689,
				UserName: "6543",
				Content:  "gitea",
			},
			{
				UserID:   689,
				UserName: "6543",
				Content:  "laugh",
			},
		},
		Closed: &closed4,
	}, issues[0])
	assertEqualIssue(t, &base.Issue{
		Number:      2,
		Title:       "Spam",
		Content:     ":(",
		Milestone:   "",
		PosterID:    689,
		PosterName:  "6543",
		PosterEmail: "6543@noreply.gitea.io",
		State:       "closed",
		IsLocked:    false,
		Created:     time.Unix(1598919780, 0),
		Updated:     closed2,
		Labels: []*base.Label{{
			Name:        "Invalid",
			Color:       "d4c5f9",
			Description: "",
		}},
		Reactions: nil,
		Closed:    &closed2,
	}, issues[1])

	/*
		ToDo:
		GetComments(issueNumber int64) ([]*Comment, error)
		GetPullRequests(page, perPage int) ([]*PullRequest, bool, error)
		GetReviews(pullRequestNumber int64) ([]*Review, error)
	*/

}
