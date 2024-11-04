// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// This package contains tests for conversations indexer modules.
// All the code in this package is only used for testing.
// Do not put any production code in this package to avoid it being included in the final binary.

package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/indexer/conversations/internal"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/timeutil"

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

			// test counting
			c.SearchOptions.Paginator = &db.ListOptions{PageSize: 0}
			countResult, err := indexer.Search(context.Background(), c.SearchOptions)
			require.NoError(t, err)
			assert.Empty(t, countResult.Hits)
			assert.Equal(t, result.Total, countResult.Total)
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
			{ID: 1000},
			{ID: 1001, Comments: []string{"hi", "hello world"}},
			{ID: 1002, Comments: []string{"hi", "hello world"}},
		},
		SearchOptions: &internal.SearchOptions{
			Keyword: "hello",
		},
		ExpectedIDs:   []int64{1002, 1001},
		ExpectedTotal: 2,
	},
	{
		Name: "RepoIDs",
		ExtraData: []*internal.IndexerData{
			{ID: 1001, RepoID: 1, IsPublic: false, Comments: []string{"hi", "hello world"}},
			{ID: 1002, RepoID: 1, IsPublic: false, Comments: []string{"hi", "hello world"}},
			{ID: 1003, RepoID: 2, IsPublic: true, Comments: []string{"hi", "hello world"}},
			{ID: 1004, RepoID: 2, IsPublic: true, Comments: []string{"hi", "hello world"}},
			{ID: 1005, RepoID: 3, IsPublic: true, Comments: []string{"hi", "hello world"}},
			{ID: 1006, RepoID: 4, IsPublic: false, Comments: []string{"hi", "hello world"}},
			{ID: 1007, RepoID: 5, IsPublic: false, Comments: []string{"hi", "hello world"}},
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
			{ID: 1001, RepoID: 1, IsPublic: false, Comments: []string{"hi", "hello world"}},
			{ID: 1002, RepoID: 1, IsPublic: false, Comments: []string{"hi", "hello world"}},
			{ID: 1003, RepoID: 2, IsPublic: true, Comments: []string{"hi", "hello world"}},
			{ID: 1004, RepoID: 2, IsPublic: true, Comments: []string{"hi", "hello world"}},
			{ID: 1005, RepoID: 3, IsPublic: true, Comments: []string{"hi", "hello world"}},
			{ID: 1006, RepoID: 4, IsPublic: false, Comments: []string{"hi", "hello world"}},
			{ID: 1007, RepoID: 5, IsPublic: false, Comments: []string{"hi", "hello world"}},
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
		Name: "conversation only",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 5, len(result.Hits))
		},
	},
	{
		Name: "updated",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				PageSize: 5,
			},
			UpdatedAfterUnix:  optional.Some(int64(20)),
			UpdatedBeforeUnix: optional.Some(int64(30)),
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
			Paginator: &db.ListOptionsAll,
			SortBy:    internal.SortByCreatedDesc,
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
			Paginator: &db.ListOptionsAll,
			SortBy:    internal.SortByUpdatedDesc,
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
			Paginator: &db.ListOptionsAll,
			SortBy:    internal.SortByCommentsDesc,
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
			Paginator: &db.ListOptionsAll,
			SortBy:    internal.SortByDeadlineDesc,
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
			Paginator: &db.ListOptionsAll,
			SortBy:    internal.SortByCreatedAsc,
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
			Paginator: &db.ListOptionsAll,
			SortBy:    internal.SortByUpdatedAsc,
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
			Paginator: &db.ListOptionsAll,
			SortBy:    internal.SortByCommentsAsc,
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
			Paginator: &db.ListOptionsAll,
			SortBy:    internal.SortByDeadlineAsc,
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
		for conversationIndex := int64(1); conversationIndex <= 20; conversationIndex++ {
			id++

			comments := make([]string, id%4)
			for i := range comments {
				comments[i] = fmt.Sprintf("comment%d", i)
			}

			data = append(data, &internal.IndexerData{
				ID:           id,
				RepoID:       repoID,
				IsPublic:     repoID%2 == 0,
				Comments:     comments,
				UpdatedUnix:  timeutil.TimeStamp(id + conversationIndex),
				CreatedUnix:  timeutil.TimeStamp(id),
				DeadlineUnix: timeutil.TimeStamp(id + conversationIndex + repoID),
				CommentCount: int64(len(comments)),
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
