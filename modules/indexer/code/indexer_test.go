// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"context"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"

	_ "code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", "..", ".."),
	})
}

func testIndexer(name string, t *testing.T, indexer Indexer) {
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

		assert.NoError(t, indexer.Delete(repoID))
	})
}
