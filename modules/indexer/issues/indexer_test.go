// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", "..", ".."))
}

func TestBleveSearchIssues(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	os.RemoveAll(setting.Indexer.IssueQueueDir)
	os.RemoveAll(setting.Indexer.IssuePath)
	setting.Indexer.IssueType = "bleve"
	InitIssueIndexer(true)

	time.Sleep(5 * time.Second)

	ids, err := SearchIssuesByKeyword(1, "issue2")
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{2}, ids)

	ids, err = SearchIssuesByKeyword(1, "first")
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)

	ids, err = SearchIssuesByKeyword(1, "for")
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1, 2, 3, 5}, ids)

	ids, err = SearchIssuesByKeyword(1, "good")
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)
}

func TestDBSearchIssues(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	setting.Indexer.IssueType = "db"
	InitIssueIndexer(true)

	ids, err := SearchIssuesByKeyword(1, "issue2")
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{2}, ids)

	ids, err = SearchIssuesByKeyword(1, "first")
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)

	ids, err = SearchIssuesByKeyword(1, "for")
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1, 2, 3, 5}, ids)

	ids, err = SearchIssuesByKeyword(1, "good")
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)
}
