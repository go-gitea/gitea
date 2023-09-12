// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// This package contains tests for issues indexer modules.
// All the code in this package is only used for testing.
// Do not put any production code in this package to avoid it being included in the final binary.

package tests

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexer(t *testing.T, indexer internal.Indexer) {
	_, err := indexer.Init(context.Background())
	require.NoError(t, err)

	require.NoError(t, indexer.Ping(context.Background()))

	var (
		ids  []int64
		data = map[int64]*internal.IndexerData{}
	)
	{
		d := generateDefaultIndexerData()
		for _, v := range d {
			ids = append(ids, v.ID)
			data[v.ID] = v
		}
		require.NoError(t, indexer.Index(context.Background(), d...))
		require.NoError(t, waitData(indexer, int64(len(data))))
	}

	defer func() {
		require.NoError(t, indexer.Delete(context.Background(), ids...))
	}()

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if len(c.ExtraData) > 0 {
				require.NoError(t, indexer.Index(context.Background(), c.ExtraData...))
				for _, v := range c.ExtraData {
					data[v.ID] = v
				}
				require.NoError(t, waitData(indexer, int64(len(data))))
				defer func() {
					for _, v := range c.ExtraData {
						require.NoError(t, indexer.Delete(context.Background(), v.ID))
						delete(data, v.ID)
					}
					require.NoError(t, waitData(indexer, int64(len(data))))
				}()
			}

			result, err := indexer.Search(context.Background(), c.SearchOptions)
			require.NoError(t, err)

			if c.Expected != nil {
				c.Expected(t, data, result)
			} else {
				ids := make([]int64, 0, len(result.Hits))
				for _, hit := range result.Hits {
					ids = append(ids, hit.ID)
				}
				assert.Equal(t, c.ExpectedIDs, ids)
				assert.Equal(t, c.ExpectedTotal, result.Total)
			}
		})
	}
}

var cases = []*testIndexerCase{
	{
		Name:          "default",
		SearchOptions: &internal.SearchOptions{},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, len(data), len(result.Hits))
			assert.Equal(t, len(data), int(result.Total))
		},
	},
	{
		Name: "empty",
		SearchOptions: &internal.SearchOptions{
			Keyword: "f1dfac73-fda6-4a6b-b8a4-2408fcb8ef69",
		},
		ExpectedIDs:   []int64{},
		ExpectedTotal: 0,
	},
	{
		Name: "with limit",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			assert.Equal(t, len(data), int(result.Total))
		},
	},
	{
		Name: "Keyword",
		ExtraData: []*internal.IndexerData{
			{ID: 1000, Title: "hi hello world"},
			{ID: 1001, Content: "hi hello world"},
			{ID: 1002, Comments: []string{"hi", "hello world"}},
		},
		SearchOptions: &internal.SearchOptions{
			Keyword: "hello",
		},
		ExpectedIDs:   []int64{1002, 1001, 1000},
		ExpectedTotal: 3,
	},
	{
		Name: "RepoIDs",
		ExtraData: []*internal.IndexerData{
			{ID: 1001, Title: "hello world", RepoID: 1, IsPublic: false},
			{ID: 1002, Title: "hello world", RepoID: 1, IsPublic: false},
			{ID: 1003, Title: "hello world", RepoID: 2, IsPublic: true},
			{ID: 1004, Title: "hello world", RepoID: 2, IsPublic: true},
			{ID: 1005, Title: "hello world", RepoID: 3, IsPublic: true},
			{ID: 1006, Title: "hello world", RepoID: 4, IsPublic: false},
			{ID: 1007, Title: "hello world", RepoID: 5, IsPublic: false},
		},
		SearchOptions: &internal.SearchOptions{
			Keyword: "hello",
			RepoIDs: []int64{1, 4},
		},
		ExpectedIDs:   []int64{1006, 1002, 1001},
		ExpectedTotal: 3,
	},
	{
		Name: "RepoIDs and AllPublic",
		ExtraData: []*internal.IndexerData{
			{ID: 1001, Title: "hello world", RepoID: 1, IsPublic: false},
			{ID: 1002, Title: "hello world", RepoID: 1, IsPublic: false},
			{ID: 1003, Title: "hello world", RepoID: 2, IsPublic: true},
			{ID: 1004, Title: "hello world", RepoID: 2, IsPublic: true},
			{ID: 1005, Title: "hello world", RepoID: 3, IsPublic: true},
			{ID: 1006, Title: "hello world", RepoID: 4, IsPublic: false},
			{ID: 1007, Title: "hello world", RepoID: 5, IsPublic: false},
		},
		SearchOptions: &internal.SearchOptions{
			Keyword:   "hello",
			RepoIDs:   []int64{1, 4},
			AllPublic: true,
		},
		ExpectedIDs:   []int64{1006, 1005, 1004, 1003, 1002, 1001},
		ExpectedTotal: 6,
	},
	{
		Name: "issue only",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			IsPull: util.OptionalBoolFalse,
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.False(t, data[v.ID].IsPull)
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool { return !v.IsPull }), result.Total)
		},
	},
	{
		Name: "pull only",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			IsPull: util.OptionalBoolTrue,
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.True(t, data[v.ID].IsPull)
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool { return v.IsPull }), result.Total)
		},
	},
	{
		Name: "opened only",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			IsClosed: util.OptionalBoolFalse,
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.False(t, data[v.ID].IsClosed)
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool { return !v.IsClosed }), result.Total)
		},
	},
	{
		Name: "closed only",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			IsClosed: util.OptionalBoolTrue,
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.True(t, data[v.ID].IsClosed)
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool { return v.IsClosed }), result.Total)
		},
	},
	{
		Name: "labels",
		ExtraData: []*internal.IndexerData{
			{ID: 1000, Title: "hello a", LabelIDs: []int64{2000, 2001, 2002}},
			{ID: 1001, Title: "hello b", LabelIDs: []int64{2000, 2001}},
			{ID: 1002, Title: "hello c", LabelIDs: []int64{2000, 2001, 2003}},
			{ID: 1003, Title: "hello d", LabelIDs: []int64{2000}},
			{ID: 1004, Title: "hello e", LabelIDs: []int64{}},
		},
		SearchOptions: &internal.SearchOptions{
			Keyword:          "hello",
			IncludedLabelIDs: []int64{2000, 2001},
			ExcludedLabelIDs: []int64{2003},
		},
		ExpectedIDs:   []int64{1001, 1000},
		ExpectedTotal: 2,
	},
	{
		Name: "include any labels",
		ExtraData: []*internal.IndexerData{
			{ID: 1000, Title: "hello a", LabelIDs: []int64{2000, 2001, 2002}},
			{ID: 1001, Title: "hello b", LabelIDs: []int64{2001}},
			{ID: 1002, Title: "hello c", LabelIDs: []int64{2000, 2001, 2003}},
			{ID: 1003, Title: "hello d", LabelIDs: []int64{2002}},
			{ID: 1004, Title: "hello e", LabelIDs: []int64{}},
		},
		SearchOptions: &internal.SearchOptions{
			Keyword:             "hello",
			IncludedAnyLabelIDs: []int64{2001, 2002},
			ExcludedLabelIDs:    []int64{2003},
		},
		ExpectedIDs:   []int64{1003, 1001, 1000},
		ExpectedTotal: 3,
	},
	{
		Name: "MilestoneIDs",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			MilestoneIDs: []int64{1, 2, 6},
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.Contains(t, []int64{1, 2, 6}, data[v.ID].MilestoneID)
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool {
				return v.MilestoneID == 1 || v.MilestoneID == 2 || v.MilestoneID == 6
			}), result.Total)
		},
	},
	{
		Name: "no MilestoneIDs",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			MilestoneIDs: []int64{0},
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.Equal(t, int64(0), data[v.ID].MilestoneID)
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool {
				return v.MilestoneID == 0
			}), result.Total)
		},
	},
	{
		Name: "ProjectID",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			ProjectID: func() *int64 {
				id := int64(1)
				return &id
			}(),
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.Equal(t, int64(1), data[v.ID].ProjectID)
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool {
				return v.ProjectID == 1
			}), result.Total)
		},
	},
	{
		Name: "no ProjectID",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			ProjectID: func() *int64 {
				id := int64(0)
				return &id
			}(),
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.Equal(t, int64(0), data[v.ID].ProjectID)
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool {
				return v.ProjectID == 0
			}), result.Total)
		},
	},
	{
		Name: "ProjectBoardID",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			ProjectBoardID: func() *int64 {
				id := int64(1)
				return &id
			}(),
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.Equal(t, int64(1), data[v.ID].ProjectBoardID)
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool {
				return v.ProjectBoardID == 1
			}), result.Total)
		},
	},
	{
		Name: "no ProjectBoardID",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			ProjectBoardID: func() *int64 {
				id := int64(0)
				return &id
			}(),
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.Equal(t, int64(0), data[v.ID].ProjectBoardID)
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool {
				return v.ProjectBoardID == 0
			}), result.Total)
		},
	},
	{
		Name: "PosterID",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			PosterID: func() *int64 {
				id := int64(1)
				return &id
			}(),
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.Equal(t, int64(1), data[v.ID].PosterID)
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool {
				return v.PosterID == 1
			}), result.Total)
		},
	},
	{
		Name: "AssigneeID",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			AssigneeID: func() *int64 {
				id := int64(1)
				return &id
			}(),
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.Equal(t, int64(1), data[v.ID].AssigneeID)
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool {
				return v.AssigneeID == 1
			}), result.Total)
		},
	},
	{
		Name: "no AssigneeID",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			AssigneeID: func() *int64 {
				id := int64(0)
				return &id
			}(),
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.Equal(t, int64(0), data[v.ID].AssigneeID)
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool {
				return v.AssigneeID == 0
			}), result.Total)
		},
	},
	{
		Name: "MentionID",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			MentionID: func() *int64 {
				id := int64(1)
				return &id
			}(),
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.Contains(t, data[v.ID].MentionIDs, int64(1))
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool {
				return slices.Contains(v.MentionIDs, 1)
			}), result.Total)
		},
	},
	{
		Name: "ReviewedID",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			ReviewedID: func() *int64 {
				id := int64(1)
				return &id
			}(),
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.Contains(t, data[v.ID].ReviewedIDs, int64(1))
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool {
				return slices.Contains(v.ReviewedIDs, 1)
			}), result.Total)
		},
	},
	{
		Name: "ReviewRequestedID",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			ReviewRequestedID: func() *int64 {
				id := int64(1)
				return &id
			}(),
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.Contains(t, data[v.ID].ReviewRequestedIDs, int64(1))
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool {
				return slices.Contains(v.ReviewRequestedIDs, 1)
			}), result.Total)
		},
	},
	{
		Name: "SubscriberID",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			SubscriberID: func() *int64 {
				id := int64(1)
				return &id
			}(),
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.Contains(t, data[v.ID].SubscriberIDs, int64(1))
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool {
				return slices.Contains(v.SubscriberIDs, 1)
			}), result.Total)
		},
	},
	{
		Name: "updated",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			UpdatedAfterUnix: func() *int64 {
				var t int64 = 20
				return &t
			}(),
			UpdatedBeforeUnix: func() *int64 {
				var t int64 = 30
				return &t
			}(),
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
			for _, v := range result.Hits {
				assert.GreaterOrEqual(t, data[v.ID].UpdatedUnix, int64(20))
				assert.LessOrEqual(t, data[v.ID].UpdatedUnix, int64(30))
			}
			assert.Equal(t, countIndexerData(data, func(v *internal.IndexerData) bool {
				return data[v.ID].UpdatedUnix >= 20 && data[v.ID].UpdatedUnix <= 30
			}), result.Total)
		},
	},
	{
		Name: "SortByCreatedDesc",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				ListAll: true,
			},
			SortBy: internal.SortByCreatedDesc,
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, len(data), len(result.Hits))
			assert.Equal(t, len(data), int(result.Total))
			for i, v := range result.Hits {
				if i < len(result.Hits)-1 {
					assert.GreaterOrEqual(t, data[v.ID].CreatedUnix, data[result.Hits[i+1].ID].CreatedUnix)
				}
			}
		},
	},
	{
		Name: "SortByUpdatedDesc",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				ListAll: true,
			},
			SortBy: internal.SortByUpdatedDesc,
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, len(data), len(result.Hits))
			assert.Equal(t, len(data), int(result.Total))
			for i, v := range result.Hits {
				if i < len(result.Hits)-1 {
					assert.GreaterOrEqual(t, data[v.ID].UpdatedUnix, data[result.Hits[i+1].ID].UpdatedUnix)
				}
			}
		},
	},
	{
		Name: "SortByCommentsDesc",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				ListAll: true,
			},
			SortBy: internal.SortByCommentsDesc,
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, len(data), len(result.Hits))
			assert.Equal(t, len(data), int(result.Total))
			for i, v := range result.Hits {
				if i < len(result.Hits)-1 {
					assert.GreaterOrEqual(t, data[v.ID].CommentCount, data[result.Hits[i+1].ID].CommentCount)
				}
			}
		},
	},
	{
		Name: "SortByDeadlineDesc",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				ListAll: true,
			},
			SortBy: internal.SortByDeadlineDesc,
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, len(data), len(result.Hits))
			assert.Equal(t, len(data), int(result.Total))
			for i, v := range result.Hits {
				if i < len(result.Hits)-1 {
					assert.GreaterOrEqual(t, data[v.ID].DeadlineUnix, data[result.Hits[i+1].ID].DeadlineUnix)
				}
			}
		},
	},
	{
		Name: "SortByCreatedAsc",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				ListAll: true,
			},
			SortBy: internal.SortByCreatedAsc,
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, len(data), len(result.Hits))
			assert.Equal(t, len(data), int(result.Total))
			for i, v := range result.Hits {
				if i < len(result.Hits)-1 {
					assert.LessOrEqual(t, data[v.ID].CreatedUnix, data[result.Hits[i+1].ID].CreatedUnix)
				}
			}
		},
	},
	{
		Name: "SortByUpdatedAsc",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				ListAll: true,
			},
			SortBy: internal.SortByUpdatedAsc,
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, len(data), len(result.Hits))
			assert.Equal(t, len(data), int(result.Total))
			for i, v := range result.Hits {
				if i < len(result.Hits)-1 {
					assert.LessOrEqual(t, data[v.ID].UpdatedUnix, data[result.Hits[i+1].ID].UpdatedUnix)
				}
			}
		},
	},
	{
		Name: "SortByCommentsAsc",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				ListAll: true,
			},
			SortBy: internal.SortByCommentsAsc,
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, len(data), len(result.Hits))
			assert.Equal(t, len(data), int(result.Total))
			for i, v := range result.Hits {
				if i < len(result.Hits)-1 {
					assert.LessOrEqual(t, data[v.ID].CommentCount, data[result.Hits[i+1].ID].CommentCount)
				}
			}
		},
	},
	{
		Name: "SortByDeadlineAsc",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				ListAll: true,
			},
			SortBy: internal.SortByDeadlineAsc,
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, len(data), len(result.Hits))
			assert.Equal(t, len(data), int(result.Total))
			for i, v := range result.Hits {
				if i < len(result.Hits)-1 {
					assert.LessOrEqual(t, data[v.ID].DeadlineUnix, data[result.Hits[i+1].ID].DeadlineUnix)
				}
			}
		},
	},
}

type testIndexerCase struct {
	Name      string
	ExtraData []*internal.IndexerData

	SearchOptions *internal.SearchOptions

	Expected      func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) // if nil, use ExpectedIDs, ExpectedTotal
	ExpectedIDs   []int64
	ExpectedTotal int64
}

func generateDefaultIndexerData() []*internal.IndexerData {
	var id int64
	var data []*internal.IndexerData
	for repoID := int64(1); repoID <= 10; repoID++ {
		for issueIndex := int64(1); issueIndex <= 20; issueIndex++ {
			id++

			comments := make([]string, id%4)
			for i := range comments {
				comments[i] = fmt.Sprintf("comment%d", i)
			}

			labelIDs := make([]int64, id%5)
			for i := range labelIDs {
				labelIDs[i] = int64(i) + 1 // LabelID should not be 0
			}
			mentionIDs := make([]int64, id%6)
			for i := range mentionIDs {
				mentionIDs[i] = int64(i) + 1 // MentionID should not be 0
			}
			reviewedIDs := make([]int64, id%7)
			for i := range reviewedIDs {
				reviewedIDs[i] = int64(i) + 1 // ReviewID should not be 0
			}
			reviewRequestedIDs := make([]int64, id%8)
			for i := range reviewRequestedIDs {
				reviewRequestedIDs[i] = int64(i) + 1 // ReviewRequestedID should not be 0
			}
			subscriberIDs := make([]int64, id%9)
			for i := range subscriberIDs {
				subscriberIDs[i] = int64(i) + 1 // SubscriberID should not be 0
			}

			data = append(data, &internal.IndexerData{
				ID:                 id,
				RepoID:             repoID,
				IsPublic:           repoID%2 == 0,
				Title:              fmt.Sprintf("issue%d of repo%d", issueIndex, repoID),
				Content:            fmt.Sprintf("content%d", issueIndex),
				Comments:           comments,
				IsPull:             issueIndex%2 == 0,
				IsClosed:           issueIndex%3 == 0,
				LabelIDs:           labelIDs,
				NoLabel:            len(labelIDs) == 0,
				MilestoneID:        issueIndex % 4,
				ProjectID:          issueIndex % 5,
				ProjectBoardID:     issueIndex % 6,
				PosterID:           id%10 + 1, // PosterID should not be 0
				AssigneeID:         issueIndex % 10,
				MentionIDs:         mentionIDs,
				ReviewedIDs:        reviewedIDs,
				ReviewRequestedIDs: reviewRequestedIDs,
				SubscriberIDs:      subscriberIDs,
				UpdatedUnix:        timeutil.TimeStamp(id + issueIndex),
				CreatedUnix:        timeutil.TimeStamp(id),
				DeadlineUnix:       timeutil.TimeStamp(id + issueIndex + repoID),
				CommentCount:       int64(len(comments)),
			})
		}
	}

	return data
}

func countIndexerData(data map[int64]*internal.IndexerData, f func(v *internal.IndexerData) bool) int64 {
	var count int64
	for _, v := range data {
		if f(v) {
			count++
		}
	}
	return count
}

// waitData waits for the indexer to index all data.
// Some engines like Elasticsearch index data asynchronously, so we need to wait for a while.
func waitData(indexer internal.Indexer, total int64) error {
	var actual int64
	for i := 0; i < 100; i++ {
		result, err := indexer.Search(context.Background(), &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 0,
			},
		})
		if err != nil {
			return err
		}
		actual = result.Total
		if actual == total {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("waitData: expected %d, actual %d", total, actual)
}
