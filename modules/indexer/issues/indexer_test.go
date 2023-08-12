// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"path"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/indexer/issues/bleve"
	"code.gitea.io/gitea/modules/setting"

	_ "code.gitea.io/gitea/models"

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

func TestDBSearchIssues(t *testing.T) {
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
