// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/indexer/issues/bleve"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/activities"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", "..", ".."),
	})
}

func TestBleveSearchIssues(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	setting.CfgProvider, _ = setting.NewConfigProviderFromData("")

	tmpIndexerDir := t.TempDir()

	setting.CfgProvider.Section("queue.issue_indexer").Key("DATADIR").MustString(path.Join(tmpIndexerDir, "issues.queue"))

	oldIssuePath := setting.Indexer.IssuePath
	setting.Indexer.IssuePath = path.Join(tmpIndexerDir, "issues.queue")
	defer func() {
		setting.Indexer.IssuePath = oldIssuePath
	}()

	setting.Indexer.IssueType = "bleve"
	setting.LoadQueueSettings()
	InitIssueIndexer(true)
	defer func() {
		if bleveIndexer, ok := (*globalIndexer.Load()).(*bleve.Indexer); ok {
			bleveIndexer.Close()
		}
	}()

	time.Sleep(5 * time.Second)

	t.Run("issue2", func(t *testing.T) {
		ids, _, err := SearchIssues(context.TODO(), &SearchOptions{
			Keyword: "issue2",
			RepoIDs: []int64{1},
		})
		assert.NoError(t, err)
		assert.EqualValues(t, []int64{2}, ids)
	})

	t.Run("first", func(t *testing.T) {
		ids, _, err := SearchIssues(context.TODO(), &SearchOptions{
			Keyword: "first",
			RepoIDs: []int64{1},
		})
		assert.NoError(t, err)
		assert.EqualValues(t, []int64{1}, ids)
	})

	t.Run("for", func(t *testing.T) {
		ids, _, err := SearchIssues(context.TODO(), &SearchOptions{
			Keyword: "for",
			RepoIDs: []int64{1},
		})
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{1, 2, 3, 5, 11}, ids)
	})

	t.Run("good", func(t *testing.T) {
		ids, _, err := SearchIssues(context.TODO(), &SearchOptions{
			Keyword: "good",
			RepoIDs: []int64{1},
		})
		assert.NoError(t, err)
		assert.EqualValues(t, []int64{1}, ids)
	})
}

func TestDBSearchIssuesWithKeyword(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	setting.Indexer.IssueType = "db"
	InitIssueIndexer(true)

	t.Run("issue2", func(t *testing.T) {
		ids, _, err := SearchIssues(context.TODO(), &SearchOptions{
			Keyword: "issue2",
			RepoIDs: []int64{1},
		})
		assert.NoError(t, err)
		assert.EqualValues(t, []int64{2}, ids)
	})

	t.Run("first", func(t *testing.T) {
		ids, _, err := SearchIssues(context.TODO(), &SearchOptions{
			Keyword: "first",
			RepoIDs: []int64{1},
		})
		assert.NoError(t, err)
		assert.EqualValues(t, []int64{1}, ids)
	})

	t.Run("for", func(t *testing.T) {
		ids, _, err := SearchIssues(context.TODO(), &SearchOptions{
			Keyword: "for",
			RepoIDs: []int64{1},
		})
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{1, 2, 3, 5, 11}, ids)
	})

	t.Run("good", func(t *testing.T) {
		ids, _, err := SearchIssues(context.TODO(), &SearchOptions{
			Keyword: "good",
			RepoIDs: []int64{1},
		})
		assert.NoError(t, err)
		assert.EqualValues(t, []int64{1}, ids)
	})
}

// TODO: add more tests
func TestDBSearchIssueWithoutKeyword(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	setting.Indexer.IssueType = "db"
	InitIssueIndexer(true)

	int64Pointer := func(x int64) *int64 {
		return &x
	}
	for _, test := range []struct {
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
				RepoIDs:    []int64{1},
				AssigneeID: int64Pointer(1),
			},
			[]int64{1},
		},
		{
			SearchOptions{
				RepoIDs:  []int64{1},
				PosterID: int64Pointer(1),
			},
			[]int64{11, 3, 2, 1},
		},
		{
			SearchOptions{
				RepoIDs:  []int64{1},
				IsClosed: util.OptionalBoolFalse,
			},
			[]int64{11, 3, 2, 1},
		},
		{
			SearchOptions{
				RepoIDs:  []int64{1},
				IsClosed: util.OptionalBoolTrue,
			},
			[]int64{5},
		},
		{
			SearchOptions{
				RepoIDs: []int64{1},
			},
			[]int64{11, 5, 3, 2, 1},
		},
		{
			SearchOptions{
				RepoIDs:    []int64{1},
				AssigneeID: int64Pointer(1),
			},
			[]int64{1},
		},
		{
			SearchOptions{
				RepoIDs:  []int64{1},
				PosterID: int64Pointer(1),
			},
			[]int64{11, 3, 2, 1},
		},
	} {
		t.Run(fmt.Sprintf("%#v", test.opts), func(t *testing.T) {
			issueIDs, _, err := SearchIssues(context.TODO(), &test.opts)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, test.expectedIDs, issueIDs)
		})
	}
}
