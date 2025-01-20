// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/activities"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestDBSearchIssues(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	setting.Indexer.IssueType = "db"
	InitIssueIndexer(true)

	t.Run("search issues with keyword", searchIssueWithKeyword)
	t.Run("search issues by index", searchIssueByIndex)
	t.Run("search issues in repo", searchIssueInRepo)
	t.Run("search issues by ID", searchIssueByID)
	t.Run("search issues is pr", searchIssueIsPull)
	t.Run("search issues is closed", searchIssueIsClosed)
	t.Run("search issues is archived", searchIssueIsArchived)
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
		require.NoError(t, err)
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueByIndex(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				Keyword: "1000",
				RepoIDs: []int64{1},
			},
			[]int64{},
		},
		{
			SearchOptions{
				Keyword: "2",
				RepoIDs: []int64{1, 2, 3, 32},
			},
			[]int64{17, 12, 7, 2},
		},
		{
			SearchOptions{
				Keyword: "1",
				RepoIDs: []int64{58},
			},
			[]int64{19},
		},
	}

	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		require.NoError(t, err)
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
		require.NoError(t, err)
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueByID(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			opts: SearchOptions{
				PosterID: optional.Some(int64(1)),
			},
			expectedIDs: []int64{11, 6, 3, 2, 1},
		},
		{
			opts: SearchOptions{
				AssigneeID: optional.Some(int64(1)),
			},
			expectedIDs: []int64{6, 1},
		},
		{
			// NOTE: This tests no assignees filtering and also ToSearchOptions() to ensure it will set AssigneeID to 0 when it is passed as -1.
			opts:        *ToSearchOptions("", &issues.IssuesOptions{AssigneeID: optional.Some(db.NoConditionID)}),
			expectedIDs: []int64{22, 21, 16, 15, 14, 13, 12, 11, 20, 5, 19, 18, 10, 7, 4, 9, 8, 3, 2},
		},
		{
			opts: SearchOptions{
				MentionID: optional.Some(int64(4)),
			},
			expectedIDs: []int64{1},
		},
		{
			opts: SearchOptions{
				ReviewedID: optional.Some(int64(1)),
			},
			expectedIDs: []int64{},
		},
		{
			opts: SearchOptions{
				ReviewRequestedID: optional.Some(int64(1)),
			},
			expectedIDs: []int64{12},
		},
		{
			opts: SearchOptions{
				SubscriberID: optional.Some(int64(1)),
			},
			expectedIDs: []int64{11, 6, 5, 3, 2, 1},
		},
		{
			// issue 20 request user 15 and team 5 which user 15 belongs to
			// the review request number of issue 20 should be 1
			opts: SearchOptions{
				ReviewRequestedID: optional.Some(int64(15)),
			},
			expectedIDs: []int64{12, 20},
		},
		{
			// user 20 approved the issue 20, so return nothing
			opts: SearchOptions{
				ReviewRequestedID: optional.Some(int64(20)),
			},
			expectedIDs: []int64{},
		},
	}

	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		require.NoError(t, err)
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
				IsPull: optional.Some(false),
			},
			[]int64{17, 16, 15, 14, 13, 6, 5, 18, 10, 7, 4, 1},
		},
		{
			SearchOptions{
				IsPull: optional.Some(true),
			},
			[]int64{22, 21, 12, 11, 20, 19, 9, 8, 3, 2},
		},
	}
	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		require.NoError(t, err)
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
				IsClosed: optional.Some(false),
			},
			[]int64{22, 21, 17, 16, 15, 14, 13, 12, 11, 20, 6, 19, 18, 10, 7, 9, 8, 3, 2, 1},
		},
		{
			SearchOptions{
				IsClosed: optional.Some(true),
			},
			[]int64{5, 4},
		},
	}
	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		require.NoError(t, err)
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueIsArchived(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				IsArchived: optional.Some(false),
			},
			[]int64{22, 21, 17, 16, 15, 13, 12, 11, 20, 6, 5, 19, 18, 10, 7, 4, 9, 8, 3, 2, 1},
		},
		{
			SearchOptions{
				IsArchived: optional.Some(true),
			},
			[]int64{14},
		},
	}
	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		require.NoError(t, err)
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
		require.NoError(t, err)
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
			[]int64{22, 21, 17, 16, 15, 14, 13, 12, 11, 20, 6, 5, 19, 18, 10, 7, 4, 9, 8, 3},
		},
	}
	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		require.NoError(t, err)
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueByTime(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				UpdatedAfterUnix: optional.Some(int64(0)),
			},
			[]int64{22, 21, 17, 16, 15, 14, 13, 12, 11, 20, 6, 5, 19, 18, 10, 7, 4, 9, 8, 3, 2, 1},
		},
	}
	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		require.NoError(t, err)
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
			[]int64{1, 2, 3, 8, 9, 4, 7, 10, 18, 19, 5, 6, 20, 11, 12, 13, 14, 15, 16, 17, 21, 22},
		},
	}
	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		require.NoError(t, err)
		assert.Equal(t, test.expectedIDs, issueIDs)
	}
}

func searchIssueInProject(t *testing.T) {
	tests := []struct {
		opts        SearchOptions
		expectedIDs []int64
	}{
		{
			SearchOptions{
				ProjectID: optional.Some(int64(1)),
			},
			[]int64{5, 3, 2, 1},
		},
		{
			SearchOptions{
				ProjectColumnID: optional.Some(int64(1)),
			},
			[]int64{1},
		},
		{
			SearchOptions{
				ProjectColumnID: optional.Some(int64(0)), // issue with in default column
			},
			[]int64{2},
		},
	}
	for _, test := range tests {
		issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
		require.NoError(t, err)
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
			[]int64{22, 21, 17, 16, 15},
			22,
		},
	}
	for _, test := range tests {
		issueIDs, total, err := SearchIssues(context.TODO(), &test.opts)
		require.NoError(t, err)
		assert.Equal(t, test.expectedIDs, issueIDs)
		assert.Equal(t, test.expectedTotal, total)
	}
}
