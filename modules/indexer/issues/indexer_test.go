// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"path"
	"path/filepath"
	"testing"
	"time"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
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
		indexer := holder.get()
		if bleveIndexer, ok := indexer.(*BleveIndexer); ok {
			bleveIndexer.Close()
		}
	}()

	time.Sleep(5 * time.Second)

	total, ids, err := Search(context.TODO(), issues_model.IssuesOptions{
		RepoIDs: []int64{1},
		Keyword: "issue2",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{2}, ids)
	assert.EqualValues(t, 1, total)

	total, ids, err = Search(context.TODO(), issues_model.IssuesOptions{
		RepoIDs: []int64{1},
		Keyword: "first",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)
	assert.EqualValues(t, 1, total)

	total, ids, err = Search(context.TODO(), issues_model.IssuesOptions{
		RepoIDs: []int64{1},
		Keyword: "for",
	})
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int64{1, 2, 3, 5, 11}, ids)
	assert.EqualValues(t, 5, total)

	total, ids, err = Search(context.TODO(), issues_model.IssuesOptions{
		RepoIDs: []int64{1},
		Keyword: "good",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)
	assert.EqualValues(t, 1, total)
}
