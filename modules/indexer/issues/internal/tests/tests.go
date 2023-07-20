// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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
	t.Run("Init", func(t *testing.T) {
		if _, err := indexer.Init(nil); err != nil {
			t.Fatalf("Init failed: %v", err)
		}
	})

	t.Run("Ping", func(t *testing.T) {
		if err := indexer.Ping(nil); err != nil {
			t.Fatalf("Ping failed: %v", err)
		}
	})

	var (
		ids  []int64
		data = map[int64]*internal.IndexerData{}
	)

	t.Run("Index", func(t *testing.T) {
		d := generateIndexerData()
		for _, v := range d {
			ids = append(ids, v.ID)
			data[v.ID] = v
		}
		if err := indexer.Index(context.Background(), d...); err != nil {
			t.Fatalf("Index failed: %v", err)
		}
	})

	defer t.Run("Delete", func(t *testing.T) {
		if err := indexer.Delete(context.Background(), ids...); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
	})

	t.Run("Search", func(t *testing.T) {
		for _, c := range cases {
			t.Run(c.Name, func(t *testing.T) {
				if c.Before != nil {
					c.Before(t, data, indexer)
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

				if c.After != nil {
					c.After(t, data, indexer)
				}
			})
		}
	})
}

var cases = []*testIndexerCase{
	{
		Name:          "empty",
		SearchOptions: &internal.SearchOptions{},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, 50, len(result.Hits)) // the default limit is 50
			assert.Equal(t, len(data), int(result.Total))
		},
	},
	{
		Name: "all",
		SearchOptions: &internal.SearchOptions{
			Paginator: &db.ListOptions{
				ListAll: true,
			},
		},
		Expected: func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) {
			assert.Equal(t, len(data), len(result.Hits)) // the default limit is 50
			assert.Equal(t, len(data), int(result.Total))
		},
	},
	{
		Name: "keyword",
		Before: func(t *testing.T, _ map[int64]*internal.IndexerData, indexer internal.Indexer) {
			newData := []*internal.IndexerData{
				{ID: 1000, Title: "hi hello world"},
				{ID: 1001, Content: "hi hello world"},
				{ID: 1002, Comments: []string{"hi", "hello world"}},
			}
			assert.NoError(t, indexer.Index(context.Background(), newData...))
		},
		After: func(t *testing.T, data map[int64]*internal.IndexerData, indexer internal.Indexer) {
			assert.NoError(t, indexer.Delete(context.Background(), 1000, 1001, 1002))
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
	Name   string
	Before func(t *testing.T, data map[int64]*internal.IndexerData, indexer internal.Indexer)
	After  func(t *testing.T, data map[int64]*internal.IndexerData, indexer internal.Indexer)

	SearchOptions *internal.SearchOptions

	Expected      func(t *testing.T, data map[int64]*internal.IndexerData, result *internal.SearchResult) // if nil, use ExpectedIDs, ExpectedTotal and ExpectedImprecise
	ExpectedIDs   []int64
	ExpectedTotal int64
}

func generateIndexerData() []*internal.IndexerData {
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
