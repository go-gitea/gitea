// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package code

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", "..", ".."))
}

func TestIndexAndSearch(t *testing.T) {
	models.PrepareTestEnv(t)

	dir, err := ioutil.TempDir("", "bleve.index")
	assert.NoError(t, err)
	if err != nil {
		assert.Fail(t, "Unable to create temporary directory")
		return
	}
	defer util.RemoveAll(dir)

	setting.Indexer.RepoIndexerEnabled = true
	idx, _, err := NewBleveIndexer(dir)
	if err != nil {
		assert.Fail(t, "Unable to create indexer Error: %v", err)
		if idx != nil {
			idx.Close()
		}
		return
	}
	defer idx.Close()

	err = idx.Index(1)
	assert.NoError(t, err)

	var (
		keywords = []struct {
			Keyword string
			IDs     []int64
			Langs   int
		}{
			{
				Keyword: "Description",
				IDs:     []int64{1},
				Langs:   1,
			},
			{
				Keyword: "repo1",
				IDs:     []int64{1},
				Langs:   1,
			},
			{
				Keyword: "non-exist",
				IDs:     []int64{},
				Langs:   0,
			},
		}
	)

	for _, kw := range keywords {
		total, res, langs, err := idx.Search(nil, "", kw.Keyword, 1, 10)
		assert.NoError(t, err)
		assert.EqualValues(t, len(kw.IDs), total)

		assert.NotNil(t, langs)
		assert.Len(t, langs, kw.Langs)

		var ids = make([]int64, 0, len(res))
		for _, hit := range res {
			ids = append(ids, hit.RepoID)
		}
		assert.EqualValues(t, kw.IDs, ids)
	}
}
