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
	"code.gitea.io/gitea/modules/setting"

	_ "code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
	"gopkg.in/ini.v1"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", "..", ".."),
	})
}

func TestBleveSearchIssues(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	setting.CfgProvider = ini.Empty()

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
		indexer := holder.get()
		if bleveIndexer, ok := indexer.(*BleveIndexer); ok {
			bleveIndexer.Close()
		}
	}()

	time.Sleep(5 * time.Second)

	_, ids, err := SearchIssuesByKeyword(context.TODO(), []int64{1}, "issue2", 0, 50)
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{2}, ids)

	_, ids, err = SearchIssuesByKeyword(context.TODO(), []int64{1}, "first", 0, 50)
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)

	_, ids, err = SearchIssuesByKeyword(context.TODO(), []int64{1}, "for", 0, 50)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int64{1, 2, 3, 5, 11}, ids)

	_, ids, err = SearchIssuesByKeyword(context.TODO(), []int64{1}, "good", 0, 50)
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)
}

func TestDBSearchIssues(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	setting.Indexer.IssueType = "db"
	InitIssueIndexer(true)

	_, ids, err := SearchIssuesByKeyword(context.TODO(), []int64{1}, "issue2", 0, 50)
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{2}, ids)

	_, ids, err = SearchIssuesByKeyword(context.TODO(), []int64{1}, "first", 0, 50)
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)

	_, ids, err = SearchIssuesByKeyword(context.TODO(), []int64{1}, "for", 0, 50)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int64{1, 2, 3, 5, 11}, ids)

	_, ids, err = SearchIssuesByKeyword(context.TODO(), []int64{1}, "good", 0, 50)
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)
}
