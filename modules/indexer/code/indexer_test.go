// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"context"
	"os"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/indexer/code/bleve"
	"code.gitea.io/gitea/modules/indexer/code/elasticsearch"
	"code.gitea.io/gitea/modules/indexer/code/internal"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/activities"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func testIndexer(name string, t *testing.T, indexer internal.Indexer) {
	t.Run(name, func(t *testing.T) {
		var repoID int64 = 1
		err := index(git.DefaultContext, indexer, repoID)
		assert.NoError(t, err)
		keywords := []struct {
			RepoIDs []int64
			Keyword string
			IDs     []int64
			Langs   int
		}{
			{
				RepoIDs: nil,
				Keyword: "Description",
				IDs:     []int64{repoID},
				Langs:   1,
			},
			{
				RepoIDs: []int64{2},
				Keyword: "Description",
				IDs:     []int64{},
				Langs:   0,
			},
			{
				RepoIDs: nil,
				Keyword: "repo1",
				IDs:     []int64{repoID},
				Langs:   1,
			},
			{
				RepoIDs: []int64{2},
				Keyword: "repo1",
				IDs:     []int64{},
				Langs:   0,
			},
			{
				RepoIDs: nil,
				Keyword: "non-exist",
				IDs:     []int64{},
				Langs:   0,
			},
		}

		for _, kw := range keywords {
			t.Run(kw.Keyword, func(t *testing.T) {
				total, res, langs, err := indexer.Search(context.TODO(), kw.RepoIDs, "", kw.Keyword, 1, 10, false)
				assert.NoError(t, err)
				assert.Len(t, kw.IDs, int(total))
				assert.Len(t, langs, kw.Langs)

				ids := make([]int64, 0, len(res))
				for _, hit := range res {
					ids = append(ids, hit.RepoID)
					assert.EqualValues(t, "# repo1\n\nDescription for repo1", hit.Content)
				}
				assert.EqualValues(t, kw.IDs, ids)
			})
		}

		assert.NoError(t, indexer.Delete(context.Background(), repoID))
	})
}

func TestBleveIndexAndSearch(t *testing.T) {
	unittest.PrepareTestEnv(t)

	dir := t.TempDir()

	idx := bleve.NewIndexer(dir)
	_, err := idx.Init(context.Background())
	if err != nil {
		if idx != nil {
			idx.Close()
		}
		assert.FailNow(t, "Unable to create bleve indexer Error: %v", err)
	}
	defer idx.Close()

	testIndexer("beleve", t, idx)
}

func TestESIndexAndSearch(t *testing.T) {
	unittest.PrepareTestEnv(t)

	u := os.Getenv("TEST_INDEXER_CODE_ES_URL")
	if u == "" {
		t.SkipNow()
		return
	}

	indexer := elasticsearch.NewIndexer(u, "gitea_codes")
	if _, err := indexer.Init(context.Background()); err != nil {
		if indexer != nil {
			indexer.Close()
		}
		assert.FailNow(t, "Unable to init ES indexer Error: %v", err)
	}

	defer indexer.Close()

	testIndexer("elastic_search", t, indexer)
}
