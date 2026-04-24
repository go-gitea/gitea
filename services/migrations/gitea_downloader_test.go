// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"code.gitea.io/gitea/models/unittest"
	base "code.gitea.io/gitea/modules/migration"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGiteaDownloadRepo(t *testing.T) {
	token := os.Getenv("GITEA_TEST_OFFICIAL_SITE_TOKEN")
	liveMode := token != ""

	_, callerFile, _, _ := runtime.Caller(0)
	fixtureDir := filepath.Join(filepath.Dir(callerFile), "_mock_data/TestGiteaDownloadRepo")
	mockServer := unittest.NewMockWebServer(t, "https://gitea.com", fixtureDir, liveMode)

	ctx := t.Context()
	downloader, err := NewGiteaDownloader(ctx, mockServer.URL, "gitea/test_repo", "", "", token)
	require.NoError(t, err, "NewGiteaDownloader error occur")
	require.NotNil(t, downloader, "NewGiteaDownloader is nil")

	repo, err := downloader.GetRepoInfo(ctx)
	assert.NoError(t, err)
	assertRepositoryEqual(t, &base.Repository{
		Name:          "test_repo",
		Owner:         "gitea",
		IsPrivate:     false,
		Description:   "Test repository for testing migration from gitea to gitea",
		CloneURL:      mockServer.URL + "/gitea/test_repo.git",
		OriginalURL:   mockServer.URL + "/gitea/test_repo",
		DefaultBranch: "master",
	}, repo)

	topics, err := downloader.GetTopics(ctx)
	assert.NoError(t, err)
	sort.Strings(topics)
	assert.Equal(t, []string{"ci", "gitea", "migration", "test"}, topics)

	labels, err := downloader.GetLabels(ctx)
	assert.NoError(t, err)
	assertLabelsEqual(t, []*base.Label{
		{
			Name:  "Bug",
			Color: "e11d21",
		},
		{
			Name:  "Enhancement",
			Color: "207de5",
		},
		{
			Name:        "Feature",
			Color:       "0052cc",
			Description: "a feature request",
		},
		{
			Name:  "Invalid",
			Color: "d4c5f9",
		},
		{
			Name:  "Question",
			Color: "fbca04",
		},
		{
			Name:  "Valid",
			Color: "53e917",
		},
	}, labels)

	milestones, err := downloader.GetMilestones(ctx)
	assert.NoError(t, err)
	assertMilestonesEqual(t, []*base.Milestone{
		{
			Title:       "V1",
			Description: "Generate Content",
			Created:     time.Unix(0, 0),
			Updated:     new(time.Unix(0, 0)),
			Closed:      new(time.Date(2020, 9, 1, 18, 36, 46, 0, time.UTC)),
			State:       "closed",
		},
		{
			Title:    "V2 Finalize",
			Created:  time.Unix(0, 0),
			Deadline: new(time.Date(2020, 9, 4, 23, 59, 59, 0, time.UTC)),
			Updated:  new(time.Date(2022, 11, 13, 5, 29, 15, 0, time.UTC)),
			State:    "open",
		},
	}, milestones)

	releases, err := downloader.GetReleases(ctx)
	assert.NoError(t, err)
	assertReleasesEqual(t, []*base.Release{
		{
			Name:            "Second Release",
			TagName:         "v2-rc1",
			TargetCommitish: "master",
			Body:            "this repo has:\r\n* reactions\r\n* wiki\r\n* issues  (open/closed)\r\n* pulls (open/closed/merged) (external/internal)\r\n* pull reviews\r\n* projects\r\n* milestones\r\n* labels\r\n* releases\r\n\r\nto test migration against",
			Draft:           false,
			Prerelease:      true,
			Created:         time.Date(2020, 9, 1, 18, 2, 43, 0, time.UTC),
			Published:       time.Date(2020, 9, 1, 18, 2, 43, 0, time.UTC),
			PublisherID:     689,
			PublisherName:   "6543",
			PublisherEmail:  "689+6543@noreply.gitea.com",
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
			PublisherEmail:  "689+6543@noreply.gitea.com",
		},
	}, releases)

	issues, isEnd, err := downloader.GetIssues(ctx, 1, 50)
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assert.Len(t, issues, 7)
	assert.Equal(t, "open", issues[0].State)

	issues, isEnd, err = downloader.GetIssues(ctx, 3, 2)
	assert.NoError(t, err)
	assert.False(t, isEnd)

	assertIssuesEqual(t, []*base.Issue{
		{
			Number:      4,
			Title:       "what is this repo about?",
			Content:     "",
			Milestone:   "V1",
			PosterID:    -1,
			PosterName:  "Ghost",
			PosterEmail: "-1+ghost@noreply.gitea.com",
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
			Closed: new(time.Date(2020, 9, 1, 15, 49, 34, 0, time.UTC)),
		},
		{
			Number:      2,
			Title:       "Spam",
			Content:     ":(",
			Milestone:   "",
			PosterID:    689,
			PosterName:  "6543",
			PosterEmail: "689+6543@noreply.gitea.com",
			State:       "closed",
			IsLocked:    false,
			Created:     time.Unix(1598919780, 0),
			Updated:     time.Unix(1598969497, 0),
			Labels: []*base.Label{{
				Name:        "Invalid",
				Color:       "d4c5f9",
				Description: "",
			}},
			Closed: new(time.Unix(1598969497, 0)),
		},
	}, issues)

	comments, _, err := downloader.GetComments(ctx, &base.Issue{Number: 4, ForeignIndex: 4})
	assert.NoError(t, err)
	assertCommentsEqual(t, []*base.Comment{
		{
			IssueIndex:  4,
			PosterID:    689,
			PosterName:  "6543",
			PosterEmail: "689+6543@noreply.gitea.com",
			Created:     time.Unix(1598975370, 0),
			Updated:     time.Unix(1599070865, 0),
			Content:     "a really good question!\n\nIt is the used as TESTSET for gitea2gitea repo migration function",
		},
		{
			IssueIndex:  4,
			PosterID:    -1,
			PosterName:  "Ghost",
			PosterEmail: "-1+ghost@noreply.gitea.com",
			Created:     time.Unix(1598975393, 0),
			Updated:     time.Unix(1598975393, 0),
			Content:     "Oh!",
		},
	}, comments)

	prs, isEnd, err := downloader.GetPullRequests(ctx, 1, 50)
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assert.Len(t, prs, 6)
	prs, isEnd, err = downloader.GetPullRequests(ctx, 1, 3)
	assert.NoError(t, err)
	assert.False(t, isEnd)
	assert.Len(t, prs, 3)
	assertPullRequestEqual(t, &base.PullRequest{
		Number:      12,
		PosterID:    689,
		PosterName:  "6543",
		PosterEmail: "689+6543@noreply.gitea.com",
		Title:       "Dont Touch",
		Content:     "\r\nadd dont touch note",
		Milestone:   "V2 Finalize",
		State:       "closed",
		IsLocked:    false,
		Created:     time.Unix(1598982759, 0),
		Updated:     time.Unix(1599023425, 0),
		Closed:      new(time.Unix(1598982933, 0)),
		Assignees:   []string{"techknowlogick"},
		Base: base.PullRequestBranch{
			CloneURL:  "",
			Ref:       "master",
			SHA:       "827aa28a907853e5ddfa40c8f9bc52471a2685fd",
			RepoName:  "test_repo",
			OwnerName: "gitea",
		},
		Head: base.PullRequestBranch{
			CloneURL:  mockServer.URL + "/6543-forks/test_repo.git",
			Ref:       "refs/pull/12/head",
			SHA:       "b6ab5d9ae000b579a5fff03f92c486da4ddf48b6",
			RepoName:  "test_repo",
			OwnerName: "6543-forks",
		},
		Merged:         true,
		MergedTime:     new(time.Unix(1598982934, 0)),
		MergeCommitSHA: "827aa28a907853e5ddfa40c8f9bc52471a2685fd",
		PatchURL:       mockServer.URL + "/gitea/test_repo/pulls/12.patch",
	}, prs[1])

	reviews, err := downloader.GetReviews(ctx, &base.Issue{Number: 7, ForeignIndex: 7})
	assert.NoError(t, err)
	assertReviewsEqual(t, []*base.Review{
		{
			ID:           1770,
			IssueIndex:   7,
			ReviewerID:   689,
			ReviewerName: "6543",
			CommitID:     "187ece0cb6631e2858a6872e5733433bb3ca3b03",
			CreatedAt:    time.Date(2020, 9, 1, 16, 12, 58, 0, time.UTC),
			State:        "COMMENT", // TODO
			Comments: []*base.ReviewComment{
				{
					ID:        116561,
					InReplyTo: 0,
					Content:   "is one `\\newline` to less?",
					TreePath:  "README.md",
					DiffHunk:  "@@ -2,3 +2,3 @@\n \n-Test repository for testing migration from gitea 2 gitea\n\\ No newline at end of file\n+Test repository for testing migration from gitea 2 gitea",
					Position:  0,
					Line:      4,
					CommitID:  "187ece0cb6631e2858a6872e5733433bb3ca3b03",
					PosterID:  689,
					Reactions: nil,
					CreatedAt: time.Date(2020, 9, 1, 16, 12, 58, 0, time.UTC),
					UpdatedAt: time.Date(2024, 6, 3, 1, 18, 36, 0, time.UTC),
				},
			},
		},
		{
			ID:           1771,
			IssueIndex:   7,
			ReviewerID:   9,
			ReviewerName: "techknowlogick",
			CommitID:     "187ece0cb6631e2858a6872e5733433bb3ca3b03",
			CreatedAt:    time.Date(2020, 9, 1, 17, 6, 47, 0, time.UTC),
			State:        "REQUEST_CHANGES", // TODO
			Content:      "I think this needs some changes",
		},
		{
			ID:           1772,
			IssueIndex:   7,
			ReviewerID:   9,
			ReviewerName: "techknowlogick",
			CommitID:     "187ece0cb6631e2858a6872e5733433bb3ca3b03",
			CreatedAt:    time.Date(2020, 9, 1, 17, 19, 51, 0, time.UTC),
			State:        base.ReviewStateApproved,
			Official:     true,
			Content:      "looks good",
		},
	}, reviews)
}
