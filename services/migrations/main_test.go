// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models/unittest"
	base "code.gitea.io/gitea/modules/migration"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func assertTimeEqual(t *testing.T, expected, actual time.Time) {
	assert.Equal(t, expected.UTC(), actual.UTC())
}

func assertTimePtrEqual(t *testing.T, expected, actual *time.Time) {
	if expected == nil {
		assert.Nil(t, actual)
	} else {
		assert.NotNil(t, actual)
		assertTimeEqual(t, *expected, *actual)
	}
}

func assertCommentEqual(t *testing.T, expected, actual *base.Comment) {
	assert.Equal(t, expected.IssueIndex, actual.IssueIndex)
	assert.Equal(t, expected.PosterID, actual.PosterID)
	assert.Equal(t, expected.PosterName, actual.PosterName)
	assert.Equal(t, expected.PosterEmail, actual.PosterEmail)
	assertTimeEqual(t, expected.Created, actual.Created)
	assertTimeEqual(t, expected.Updated, actual.Updated)
	assert.Equal(t, expected.Content, actual.Content)
	assertReactionsEqual(t, expected.Reactions, actual.Reactions)
}

func assertCommentsEqual(t *testing.T, expected, actual []*base.Comment) {
	if assert.Len(t, actual, len(expected)) {
		for i := range expected {
			assertCommentEqual(t, expected[i], actual[i])
		}
	}
}

func assertLabelEqual(t *testing.T, expected, actual *base.Label) {
	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.Exclusive, actual.Exclusive)
	assert.Equal(t, expected.Color, actual.Color)
	assert.Equal(t, expected.Description, actual.Description)
}

func assertLabelsEqual(t *testing.T, expected, actual []*base.Label) {
	if assert.Len(t, actual, len(expected)) {
		for i := range expected {
			assertLabelEqual(t, expected[i], actual[i])
		}
	}
}

func assertMilestoneEqual(t *testing.T, expected, actual *base.Milestone) {
	assert.Equal(t, expected.Title, actual.Title)
	assert.Equal(t, expected.Description, actual.Description)
	assertTimePtrEqual(t, expected.Deadline, actual.Deadline)
	assertTimeEqual(t, expected.Created, actual.Created)
	assertTimePtrEqual(t, expected.Updated, actual.Updated)
	assertTimePtrEqual(t, expected.Closed, actual.Closed)
	assert.Equal(t, expected.State, actual.State)
}

func assertMilestonesEqual(t *testing.T, expected, actual []*base.Milestone) {
	if assert.Len(t, actual, len(expected)) {
		for i := range expected {
			assertMilestoneEqual(t, expected[i], actual[i])
		}
	}
}

func assertIssueEqual(t *testing.T, expected, actual *base.Issue) {
	assert.Equal(t, expected.Number, actual.Number)
	assert.Equal(t, expected.PosterID, actual.PosterID)
	assert.Equal(t, expected.PosterName, actual.PosterName)
	assert.Equal(t, expected.PosterEmail, actual.PosterEmail)
	assert.Equal(t, expected.Title, actual.Title)
	assert.Equal(t, expected.Content, actual.Content)
	assert.Equal(t, expected.Ref, actual.Ref)
	assert.Equal(t, expected.Milestone, actual.Milestone)
	assert.Equal(t, expected.State, actual.State)
	assert.Equal(t, expected.IsLocked, actual.IsLocked)
	assertTimeEqual(t, expected.Created, actual.Created)
	assertTimeEqual(t, expected.Updated, actual.Updated)
	assertTimePtrEqual(t, expected.Closed, actual.Closed)
	assertLabelsEqual(t, expected.Labels, actual.Labels)
	assertReactionsEqual(t, expected.Reactions, actual.Reactions)
	assert.ElementsMatch(t, expected.Assignees, actual.Assignees)
}

func assertIssuesEqual(t *testing.T, expected, actual []*base.Issue) {
	if assert.Len(t, actual, len(expected)) {
		for i := range expected {
			assertIssueEqual(t, expected[i], actual[i])
		}
	}
}

func assertPullRequestEqual(t *testing.T, expected, actual *base.PullRequest) {
	assert.Equal(t, expected.Number, actual.Number)
	assert.Equal(t, expected.Title, actual.Title)
	assert.Equal(t, expected.PosterID, actual.PosterID)
	assert.Equal(t, expected.PosterName, actual.PosterName)
	assert.Equal(t, expected.PosterEmail, actual.PosterEmail)
	assert.Equal(t, expected.Content, actual.Content)
	assert.Equal(t, expected.Milestone, actual.Milestone)
	assert.Equal(t, expected.State, actual.State)
	assertTimeEqual(t, expected.Created, actual.Created)
	assertTimeEqual(t, expected.Updated, actual.Updated)
	assertTimePtrEqual(t, expected.Closed, actual.Closed)
	assertLabelsEqual(t, expected.Labels, actual.Labels)
	assert.Equal(t, expected.PatchURL, actual.PatchURL)
	assert.Equal(t, expected.Merged, actual.Merged)
	assertTimePtrEqual(t, expected.MergedTime, actual.MergedTime)
	assert.Equal(t, expected.MergeCommitSHA, actual.MergeCommitSHA)
	assertPullRequestBranchEqual(t, expected.Head, actual.Head)
	assertPullRequestBranchEqual(t, expected.Base, actual.Base)
	assert.ElementsMatch(t, expected.Assignees, actual.Assignees)
	assert.Equal(t, expected.IsLocked, actual.IsLocked)
	assertReactionsEqual(t, expected.Reactions, actual.Reactions)
}

func assertPullRequestsEqual(t *testing.T, expected, actual []*base.PullRequest) {
	if assert.Len(t, actual, len(expected)) {
		for i := range expected {
			assertPullRequestEqual(t, expected[i], actual[i])
		}
	}
}

func assertPullRequestBranchEqual(t *testing.T, expected, actual base.PullRequestBranch) {
	assert.Equal(t, expected.CloneURL, actual.CloneURL)
	assert.Equal(t, expected.Ref, actual.Ref)
	assert.Equal(t, expected.SHA, actual.SHA)
	assert.Equal(t, expected.RepoName, actual.RepoName)
	assert.Equal(t, expected.OwnerName, actual.OwnerName)
}

func assertReactionEqual(t *testing.T, expected, actual *base.Reaction) {
	assert.Equal(t, expected.UserID, actual.UserID)
	assert.Equal(t, expected.UserName, actual.UserName)
	assert.Equal(t, expected.Content, actual.Content)
}

func assertReactionsEqual(t *testing.T, expected, actual []*base.Reaction) {
	if assert.Len(t, actual, len(expected)) {
		for i := range expected {
			assertReactionEqual(t, expected[i], actual[i])
		}
	}
}

func assertReleaseAssetEqual(t *testing.T, expected, actual *base.ReleaseAsset) {
	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.ContentType, actual.ContentType)
	assert.Equal(t, expected.Size, actual.Size)
	assert.Equal(t, expected.DownloadCount, actual.DownloadCount)
	assertTimeEqual(t, expected.Created, actual.Created)
	assertTimeEqual(t, expected.Updated, actual.Updated)
	assert.Equal(t, expected.DownloadURL, actual.DownloadURL)
}

func assertReleaseAssetsEqual(t *testing.T, expected, actual []*base.ReleaseAsset) {
	if assert.Len(t, actual, len(expected)) {
		for i := range expected {
			assertReleaseAssetEqual(t, expected[i], actual[i])
		}
	}
}

func assertReleaseEqual(t *testing.T, expected, actual *base.Release) {
	assert.Equal(t, expected.TagName, actual.TagName)
	assert.Equal(t, expected.TargetCommitish, actual.TargetCommitish)
	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.Body, actual.Body)
	assert.Equal(t, expected.Draft, actual.Draft)
	assert.Equal(t, expected.Prerelease, actual.Prerelease)
	assert.Equal(t, expected.PublisherID, actual.PublisherID)
	assert.Equal(t, expected.PublisherName, actual.PublisherName)
	assert.Equal(t, expected.PublisherEmail, actual.PublisherEmail)
	assertReleaseAssetsEqual(t, expected.Assets, actual.Assets)
	assertTimeEqual(t, expected.Created, actual.Created)
	assertTimeEqual(t, expected.Published, actual.Published)
}

func assertReleasesEqual(t *testing.T, expected, actual []*base.Release) {
	if assert.Len(t, actual, len(expected)) {
		for i := range expected {
			assertReleaseEqual(t, expected[i], actual[i])
		}
	}
}

func assertRepositoryEqual(t *testing.T, expected, actual *base.Repository) {
	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.Owner, actual.Owner)
	assert.Equal(t, expected.IsPrivate, actual.IsPrivate)
	assert.Equal(t, expected.IsMirror, actual.IsMirror)
	assert.Equal(t, expected.Description, actual.Description)
	assert.Equal(t, expected.CloneURL, actual.CloneURL)
	assert.Equal(t, expected.OriginalURL, actual.OriginalURL)
	assert.Equal(t, expected.DefaultBranch, actual.DefaultBranch)
}

func assertReviewEqual(t *testing.T, expected, actual *base.Review) {
	assert.Equal(t, expected.ID, actual.ID, "ID")
	assert.Equal(t, expected.IssueIndex, actual.IssueIndex, "IsssueIndex")
	assert.Equal(t, expected.ReviewerID, actual.ReviewerID, "ReviewerID")
	assert.Equal(t, expected.ReviewerName, actual.ReviewerName, "ReviewerName")
	assert.Equal(t, expected.Official, actual.Official, "Official")
	assert.Equal(t, expected.CommitID, actual.CommitID, "CommitID")
	assert.Equal(t, expected.Content, actual.Content, "Content")
	assert.WithinDuration(t, expected.CreatedAt, actual.CreatedAt, 10*time.Second)
	assert.Equal(t, expected.State, actual.State, "State")
	assertReviewCommentsEqual(t, expected.Comments, actual.Comments)
}

func assertReviewsEqual(t *testing.T, expected, actual []*base.Review) {
	if assert.Len(t, actual, len(expected)) {
		for i := range expected {
			assertReviewEqual(t, expected[i], actual[i])
		}
	}
}

func assertReviewCommentEqual(t *testing.T, expected, actual *base.ReviewComment) {
	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.InReplyTo, actual.InReplyTo)
	assert.Equal(t, expected.Content, actual.Content)
	assert.Equal(t, expected.TreePath, actual.TreePath)
	assert.Equal(t, expected.DiffHunk, actual.DiffHunk)
	assert.Equal(t, expected.Position, actual.Position)
	assert.Equal(t, expected.Line, actual.Line)
	assert.Equal(t, expected.CommitID, actual.CommitID)
	assert.Equal(t, expected.PosterID, actual.PosterID)
	assertReactionsEqual(t, expected.Reactions, actual.Reactions)
	assertTimeEqual(t, expected.CreatedAt, actual.CreatedAt)
	assertTimeEqual(t, expected.UpdatedAt, actual.UpdatedAt)
}

func assertReviewCommentsEqual(t *testing.T, expected, actual []*base.ReviewComment) {
	if assert.Len(t, actual, len(expected)) {
		for i := range expected {
			assertReviewCommentEqual(t, expected[i], actual[i])
		}
	}
}
