// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	_ "code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"gopkg.in/ini.v1"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, filepath.Join("..", "..", ".."))
}

func TestBleveSearchIssues(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	setting.Cfg = ini.Empty()

	tmpIndexerDir, err := os.MkdirTemp("", "issues-indexer")
	if err != nil {
		assert.Fail(t, "Unable to create temporary directory: %v", err)
		return
	}
	oldQueueDir := setting.Indexer.IssueQueueDir
	oldIssuePath := setting.Indexer.IssuePath
	setting.Indexer.IssueQueueDir = path.Join(tmpIndexerDir, "issues.queue")
	setting.Indexer.IssuePath = path.Join(tmpIndexerDir, "issues.queue")
	defer func() {
		setting.Indexer.IssueQueueDir = oldQueueDir
		setting.Indexer.IssuePath = oldIssuePath
		util.RemoveAll(tmpIndexerDir)
	}()

	setting.Indexer.IssueType = "bleve"
	setting.NewQueueService()
	InitIssueIndexer(true)
	defer func() {
		indexer := holder.get()
		if bleveIndexer, ok := indexer.(*BleveIndexer); ok {
			bleveIndexer.Close()
		}
	}()

	time.Sleep(5 * time.Second)

	ids, err := SearchIssuesByKeyword([]int64{1}, "issue2")
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{2}, ids)

	ids, err = SearchIssuesByKeyword([]int64{1}, "first")
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)

	ids, err = SearchIssuesByKeyword([]int64{1}, "for")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int64{1, 2, 3, 5, 11}, ids)

	ids, err = SearchIssuesByKeyword([]int64{1}, "good")
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)

}

func TestDBSearchIssues(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	setting.Indexer.IssueType = "db"
	InitIssueIndexer(true)

	ids, err := SearchIssuesByKeyword([]int64{1}, "issue2")
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{2}, ids)

	ids, err = SearchIssuesByKeyword([]int64{1}, "first")
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)

	ids, err = SearchIssuesByKeyword([]int64{1}, "for")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int64{1, 2, 3, 5, 11}, ids)

	ids, err = SearchIssuesByKeyword([]int64{1}, "good")
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)
}
