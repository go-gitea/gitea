// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// This package contains tests for the indexer module.
// All the code in this package is only used for testing.
// Do not put any production code in this package to avoid it being included in the final binary.

package tests

import (
	"context"
	"fmt"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
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
				defer func() {
					for _, v := range c.ExtraData {
						require.NoError(t, indexer.Delete(context.Background(), v.ID))
						delete(data, v.ID)
					}
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
			if result.Imprecise {
				// If an engine does not support complex queries, do not use TestIndexer to test it
				t.Errorf("Expected imprecise to be false, got true")
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
		Name: "keyword",
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
	// TODO: add more cases
}

type testIndexerCase struct {
	Name      string
	ExtraData []*internal.IndexerData

	SearchOptions *internal.SearchOptions

	Expected      func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) // if nil, use ExpectedIDs, ExpectedTotal and ExpectedImprecise
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
				labelIDs[i] = int64(i)
			}
			mentionIDs := make([]int64, id%6)
			for i := range mentionIDs {
				mentionIDs[i] = int64(i)
			}
			reviewedIDs := make([]int64, id%7)
			for i := range reviewedIDs {
				reviewedIDs[i] = int64(i)
			}
			reviewRequestedIDs := make([]int64, id%8)
			for i := range reviewRequestedIDs {
				reviewRequestedIDs[i] = int64(i)
			}
			subscriberIDs := make([]int64, id%9)
			for i := range subscriberIDs {
				subscriberIDs[i] = int64(i)
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
				PosterID:           id % 10,
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
