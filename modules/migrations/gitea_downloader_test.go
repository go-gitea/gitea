// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"net/http"
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/migrations/base"

	"github.com/stretchr/testify/assert"

	_ "github.com/mattn/go-sqlite3"
)

func TestGiteaDownloadRepo(t *testing.T) {
	// Skip tests if Gitea token is not found
	giteaPersonalAccessToken := os.Getenv("GITEA_ACCESS_TOKEN")
	if giteaPersonalAccessToken == "" {
		t.Skip("skipped test because GITEA_ACCESS_TOKEN was not in the environment")
	}

	resp, err := http.Get("https://gitea.com/j-be/test_repo")
	if err != nil || resp.StatusCode != 200 {
		t.Skipf("Can't access test repo, skipping %s", t.Name())
	}

	downloader := NewGiteaDownloader("https://gitea.com", "j-be/test_repo", giteaPersonalAccessToken, "")
	if downloader == nil {
		t.Fatal("NewGiteaDownloader is nil")
	}
	repo, err := downloader.GetRepoInfo()
	assert.NoError(t, err)
	assert.EqualValues(t, &base.Repository{
		Name:        "test_repo",
		Owner:       "j-be",
		Description: "Test repository for testing migration from gitea to gitea",
		CloneURL:    "https://gitea.com/j-be/test_repo.git",
		OriginalURL: "https://gitea.com/j-be/test_repo",
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
			PublisherID:     5610,
			PublisherName:   "j-be",
			PublisherEmail:  "j-be@noreply.gitea.io",
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
			Number:      2,
			Title:       "Test issue",
			Content:     "This is test issue 2, do not touch!",
			Milestone:   "1.1.0",
			PosterID:    5610,
			PosterName:  "j-be",
			PosterEmail: "j-be@noreply.gitea.io",
			State:       "closed",
			Created:     time.Date(2019, 11, 12, 21, 0, 6, 0, time.UTC),
			Updated:     time.Date(2019, 11, 12, 22, 7, 14, 0, time.UTC),
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
			Closed: &closed2,
		},
		{
			Number:      1,
			Title:       "Please add an animated gif icon to the merge button",
			Content:     "I just want the merge button to hurt my eyes a little. \xF0\x9F\x98\x9D ",
			Milestone:   "1.0.0",
			PosterID:    5610,
			PosterName:  "j-be",
			PosterEmail: "j-be@noreply.gitea.io",
			State:       "closed",
			Created:     time.Date(2019, 11, 9, 17, 0, 29, 0, time.UTC),
			Updated:     time.Date(2019, 11, 12, 20, 29, 53, 0, time.UTC),
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
			Closed: &closed1,
		},
	}, issues)

	// downloader.GetComments()
	comments, err := downloader.GetComments(2)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(comments))
	assert.EqualValues(t, []*base.Comment{
		{
			IssueIndex:  2,
			PosterID:    5610,
			PosterName:  "j-be",
			PosterEmail: "j-be@noreply.gitea.io",
			Created:     time.Date(2019, 11, 12, 21, 0, 13, 0, time.UTC),
			Updated:     time.Date(2019, 11, 12, 21, 0, 13, 0, time.UTC),
			Content:     "This is a comment",
			Reactions: []*base.Reaction{
				{
					UserID:   1669571,
					UserName: "mrsdizzie",
					Content:  "+1",
				},
			},
		},
		{
			IssueIndex:  2,
			PosterID:    5610,
			PosterName:  "j-be",
			PosterEmail: "j-be@noreply.gitea.io",
			Created:     time.Date(2019, 11, 12, 22, 7, 14, 0, time.UTC),
			Updated:     time.Date(2019, 11, 12, 22, 7, 14, 0, time.UTC),
			Content:     "A second comment",
			Reactions:   nil,
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
			Number:      4,
			Title:       "Test branch",
			Content:     "do not merge this PR",
			Milestone:   "1.0.0",
			PosterID:    5610,
			PosterName:  "j-be",
			PosterEmail: "j-be@noreply.gitea.io",
			State:       "open",
			Created:     time.Date(2019, 11, 12, 21, 54, 18, 0, time.UTC),
			Updated:     time.Date(2020, 1, 4, 11, 30, 1, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:        "bug",
					Color:       "d73a4a",
					Description: "Something isn't working",
				},
			},
			PatchURL: "https://gitea.com/j-be/test_repo/pulls/4.patch",
			Head: base.PullRequestBranch{
				Ref:       "mrsdizzie/test-branch",
				SHA:       "2be9101c543658591222acbee3eb799edfc3853d",
				RepoName:  "test_repo",
				OwnerName: "j-be",
				CloneURL:  "https://gitea.com/j-be/test_repo.git",
			},
			Base: base.PullRequestBranch{
				CloneURL:  "https://gitea.com/j-be/test_repo.git",
				Ref:       "master",
				SHA:       "f32b0a9dfd09a60f616f29158f772cedd89942d2",
				OwnerName: "j-be",
				RepoName:  "test_repo",
			},
			Merged:         false,
			MergeCommitSHA: "",
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
		},
		{
			Number:      3,
			Title:       "Update README.md",
			Content:     "add warning to readme",
			Milestone:   "1.1.0",
			PosterID:    5610,
			PosterName:  "j-be",
			PosterEmail: "j-be@noreply.gitea.io",
			State:       "closed",
			Created:     time.Date(2019, 11, 12, 21, 21, 43, 0, time.UTC),
			Updated:     time.Date(2019, 11, 12, 21, 39, 28, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:        "documentation",
					Color:       "0075ca",
					Description: "Improvements or additions to documentation",
				},
			},
			PatchURL: "https://gitea.com/j-be/test_repo/pulls/3.patch",
			Head: base.PullRequestBranch{
				Ref:      "master",
				CloneURL: "https://gitea.com/j-be/test_repo.git",
				SHA:      "076160cf0b039f13e5eff19619932d181269414b",
				RepoName: "test_repo",
				OwnerName: "j-be",
			},
			Base: base.PullRequestBranch{
				CloneURL:  "https://gitea.com/j-be/test_repo.git",
				Ref:       "master",
				SHA:       "72866af952e98d02a73003501836074b286a78f6",
				OwnerName: "j-be",
				RepoName:  "test_repo",
			},
			Closed:         &closed1,
			Merged:         true,
			MergedTime:     &merged1,
			MergeCommitSHA: "f32b0a9dfd09a60f616f29158f772cedd89942d2",
		},
	}, prs)

	reviews, err := downloader.GetReviews(3)
	assert.NoError(t, err)
	assert.EqualValues(t, []*base.Review{
		{
			ID:           1408,
			IssueIndex:   3,
			ReviewerID:   0,
			ReviewerName: "jolheiser",
			CommitID:     "076160cf0b039f13e5eff19619932d181269414b",
			CreatedAt:    time.Date(2019, 11, 12, 21, 35, 24, 0, time.UTC),
			State:        base.ReviewStateApproved,
		},
		{
			ID:           1409,
			IssueIndex:   3,
			ReviewerID:   1824502,
			ReviewerName: "zeripath",
			CommitID:     "076160cf0b039f13e5eff19619932d181269414b",
			CreatedAt:    time.Date(2019, 11, 12, 21, 35, 36, 0, time.UTC),
			State:        base.ReviewStateApproved,
		},
		{
			ID:           1410,
			IssueIndex:   3,
			ReviewerID:   165205,
			ReviewerName: "lafriks",
			CommitID:     "076160cf0b039f13e5eff19619932d181269414b",
			CreatedAt:    time.Date(2019, 11, 12, 21, 38, 00, 0, time.UTC),
			State:        base.ReviewStateApproved,
		},
	}, reviews)

	reviews, err = downloader.GetReviews(4)
	assert.NoError(t, err)
	assert.EqualValues(t, []*base.Review{
		{
			ID:           1411,
			IssueIndex:   4,
			ReviewerID:   0,
			ReviewerName: "lunny",
			CommitID:     "2be9101c543658591222acbee3eb799edfc3853d",
			CreatedAt:    time.Date(2020, 01, 04, 05, 33, 18, 0, time.UTC),
			State:        base.ReviewStateApproved,
			Comments: []*base.ReviewComment{
				{
					ID:        113997,
					Content:   "This is a good pull request.",
					TreePath:  "README.md",
					DiffHunk:  "@@ -1,2 +1,4 @@\n # test_repo\n Test repository for testing migration from github to gitea\n+",
					Position:  3,
					CommitID:  "2be9101c543658591222acbee3eb799edfc3853d",
					PosterID:  81045,
					CreatedAt: time.Date(2020, 01, 04, 05, 33, 06, 0, time.UTC),
					UpdatedAt: time.Date(2020, 01, 04, 05, 33, 18, 0, time.UTC),
				},
			},
		},
		{
			ID:           1412,
			IssueIndex:   4,
			ReviewerID:   0,
			ReviewerName: "lunny",
			CommitID:     "2be9101c543658591222acbee3eb799edfc3853d",
			CreatedAt:    time.Date(2020, 01, 04, 06, 07, 06, 0, time.UTC),
			State:        base.ReviewStateChangesRequested,
			Content:      "Don't add more reviews",
		},
		{
			ID:           1413,
			IssueIndex:   4,
			ReviewerID:   81045,
			ReviewerName: "lunny",
			CommitID:     "2be9101c543658591222acbee3eb799edfc3853d",
			CreatedAt:    time.Date(2020, 01, 04, 11, 21, 41, 0, time.UTC),
			State:        base.ReviewStateCommented,
			Comments: []*base.ReviewComment{
				{
					ID:        114000,
					Content:   "test a single comment.",
					TreePath:  "LICENSE",
					DiffHunk:  "@@ -19,3 +19,5 @@ AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER\n LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,\n OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE\n SOFTWARE.\n+",
					Position:  4,
					CommitID:  "2be9101c543658591222acbee3eb799edfc3853d",
					PosterID:  81045,
					CreatedAt: time.Date(2020, 01, 04, 11, 21, 41, 0, time.UTC),
					UpdatedAt: time.Date(2020, 01, 04, 11, 21, 41, 0, time.UTC),
				},
			},
		},
	}, reviews)
}
