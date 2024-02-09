// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/activities"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestDBSearchIssues(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	setting.Indexer.IssueType = "db"
	InitIssueIndexer(true)

	t.Run("search issues with keyword", searchIssueWithKeyword)
	t.Run("search issues in repo", searchIssueInRepo)
	t.Run("search issues by ID", searchIssueByID)
	t.Run("search issues is pr", searchIssueIsPull)
	t.Run("search issues is closed", searchIssueIsClosed)
	t.Run("search issues by milestone", searchIssueByMilestoneID)
	t.Run("search issues by label", searchIssueByLabelID)
	t.Run("search issues by time", searchIssueByTime)
	t.Run("search issues with order", searchIssueWithOrder)
	t.Run("search issues in project", searchIssueInProject)
	t.Run("search issues with paginator", searchIssueWithPaginator)
}

func searchIssueWithKeyword(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				Keyword: "issue2",
				RepoIDs: []int64{1},
			},
			[]int64{2},
		},
		{
			SearchOptions{
				Keyword: "first",
				RepoIDs: []int64{1},
			},
			[]int64{1},
		},
		{
			SearchOptions{
				Keyword: "for",
				RepoIDs: []int64{1},
			},
			[]int64{11, 5, 3, 2, 1},
		},
		{
			SearchOptions{
				Keyword: "good",
				RepoIDs: []int64{1},
			},
			[]int64{1},
		},
	}

	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueInRepo(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				RepoIDs: []int64{1},
			},
			[]int64{11, 5, 3, 2, 1},
		},
		{
			SearchOptions{
				RepoIDs: []int64{2},
			},
			[]int64{7, 4},
		},
		{
			SearchOptions{
				RepoIDs: []int64{3},
			},
			[]int64{12, 6},
		},
		{
			SearchOptions{
				RepoIDs: []int64{4},
			},
			[]int64{},
		},
		{
			SearchOptions{
				RepoIDs: []int64{5},
			},
			[]int64{15},
		},
	}

	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueByID(t *testing.T) {
	int64Pointer := func(x int64) *int64 {
		return &x
	}
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				PosterID: int64Pointer(1),
			},
			[]int64{11, 6, 3, 2, 1},
		},
		{
			SearchOptions{
				AssigneeID: int64Pointer(1),
			},
			[]int64{6, 1},
		},
		{
			SearchOptions{
				MentionID: int64Pointer(4),
			},
			[]int64{1},
		},
		{
			SearchOptions{
				ReviewedID: int64Pointer(1),
			},
			[]int64{},
		},
		{
			SearchOptions{
				ReviewRequestedID: int64Pointer(1),
			},
			[]int64{12},
		},
		{
			SearchOptions{
				SubscriberID: int64Pointer(1),
			},
			[]int64{11, 6, 5, 3, 2, 1},
		},
		{
			// issue 20 request user 15 and team 5 which user 15 belongs to
			// the review request number of issue 20 should be 1
			SearchOptions{
				ReviewRequestedID: int64Pointer(15),
			},
			[]int64{12, 20},
		},
		{
			// user 20 approved the issue 20, so return nothing
			SearchOptions{
				ReviewRequestedID: int64Pointer(20),
			},
			[]int64{},
		},
	}

	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueIsPull(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				IsPull: util.OptionalBoolFalse,
			},
			[]int64{17, 16, 15, 14, 13, 6, 5, 18, 10, 7, 4, 1},
		},
		{
			SearchOptions{
				IsPull: util.OptionalBoolTrue,
			},
			[]int64{12, 11, 20, 19, 9, 8, 3, 2},
		},
	}
	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueIsClosed(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				IsClosed: util.OptionalBoolFalse,
			},
			[]int64{17, 16, 15, 14, 13, 12, 11, 20, 6, 19, 18, 10, 7, 9, 8, 3, 2, 1},
		},
		{
			SearchOptions{
				IsClosed: util.OptionalBoolTrue,
			},
			[]int64{5, 4},
		},
	}
	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueByMilestoneID(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				MilestoneIDs: []int64{1},
			},
			[]int64{2},
		},
		{
			SearchOptions{
				MilestoneIDs: []int64{3},
			},
			[]int64{3},
		},
	}
	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueByLabelID(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				IncludedLabelIDs: []int64{1},
			},
			[]int64{2, 1},
		},
		{
			SearchOptions{
				IncludedLabelIDs: []int64{4},
			},
			[]int64{2},
		},
		{
			SearchOptions{
				ExcludedLabelIDs: []int64{1},
			},
			[]int64{17, 16, 15, 14, 13, 12, 11, 20, 6, 5, 19, 18, 10, 7, 4, 9, 8, 3},
		},
	}
	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueByTime(t *testing.T) {
	int64Pointer := func(i int64) *int64 {
		return &i
	}
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				UpdatedAfterUnix: int64Pointer(0),
			},
			[]int64{17, 16, 15, 14, 13, 12, 11, 20, 6, 5, 19, 18, 10, 7, 4, 9, 8, 3, 2, 1},
		},
	}
	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueWithOrder(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				SortBy: internal.SortByCreatedAsc,
			},
			[]int64{1, 2, 3, 8, 9, 4, 7, 10, 18, 19, 5, 6, 20, 11, 12, 13, 14, 15, 16, 17},
		},
	}
	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueInProject(t *testing.T) {
	int64Pointer := func(i int64) *int64 {
		return &i
	}
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				ProjectID: int64Pointer(1),
			},
			[]int64{5, 3, 2, 1},
		},
		{
			SearchOptions{
				ProjectBoardID: int64Pointer(1),
			},
			[]int64{1},
		},
		{
			SearchOptions{
				ProjectBoardID: int64Pointer(0), // issue with in default board
			},
			[]int64{2},
		},
	}
	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueWithPaginator(t *testing.T) {
	tests := []struct {
		opts          SearchOptions
		expectedIDs   []int64
		expectedTotal int64
	}{
		{
			SearchOptions{
				Paginator: &db.ListOptions{
					PageSize: 5,
				},
			},
			[]int64{17, 16, 15, 14, 13},
			20,
		},
	}
	for _, test := range tests {
		issueIDs, total, err := SearchIssues(context.TODO(), &test.opts)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, test.expectedIDs, issueIDs)
		assert.Equal(t, test.expectedTotal, total)
	}
}
