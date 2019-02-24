// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func fatalTestError(fmtStr string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, fmtStr, args...)
	os.Exit(1)
}

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", "..", ".."))
}

func TestBleveSearchIssues(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	os.RemoveAll(setting.Indexer.IssueIndexerQueueDir)
	os.RemoveAll(setting.Indexer.IssuePath)
	setting.Indexer.IssueType = "bleve"
	if err := InitIssueIndexer(true); err != nil {
		fatalTestError("Error InitIssueIndexer: %v\n", err)
	}

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
	if err := InitIssueIndexer(true); err != nil {
		fatalTestError("Error InitIssueIndexer: %v\n", err)
	}

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
