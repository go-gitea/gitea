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

	"code.gitea.io/gitea/modules/migrations/base"

	"github.com/stretchr/testify/assert"
)

func TestGiteaDownloadRepo(t *testing.T) {
	// Skip tests if Gitea token is not found
	giteaToken := os.Getenv("GITEA_TOKEN")
	if giteaToken == "" {
		t.Skip("skipped test because GITEA_TOKEN was not in the environment")
	}

	resp, err := http.Get("https://gitea.com/gitea")
	if err != nil || resp.StatusCode != 200 {
		t.Skipf("Can't access test repo, skipping %s", t.Name())
	}

	downloader := NewGiteaDownloader("https://gitea.com", "6543/test_repo", "", "", giteaToken)
	if downloader == nil {
		t.Fatal("NewGitlabDownloader is nil")
	}

	repo, err := downloader.GetRepoInfo()
	assert.NoError(t, err)
	assert.EqualValues(t, &base.Repository{
		Name:        "test_repo",
		Owner:       "6543",
		IsPrivate:   false, // ToDo: set test repo private
		Description: "Test repository for testing migration from gitea to gitea",
		CloneURL:    "https://gitea.com/6543/test_repo.git",
		OriginalURL: "https://gitea.com/6543/test_repo",
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

	/*
		ToDo:
		GetAsset(relTag string, relID, id int64) (io.ReadCloser, error)
		GetMilestones() ([]*Milestone, error)
		GetReleases() ([]*Release, error)
		GetIssues(page, perPage int) ([]*Issue, bool, error)
		GetComments(issueNumber int64) ([]*Comment, error)
		GetPullRequests(page, perPage int) ([]*PullRequest, bool, error)
		GetReviews(pullRequestNumber int64) ([]*Review, error)
	*/

}
