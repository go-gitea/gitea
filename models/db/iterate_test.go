// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db_test

import (
	"context"
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/container"

	"github.com/stretchr/testify/assert"
)

func TestIterate(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	xe := unittest.GetXORMEngine()
	assert.NoError(t, xe.Sync(&repo_model.RepoUnit{}))

	cnt, err := db.GetEngine(t.Context()).Count(&repo_model.RepoUnit{})
	assert.NoError(t, err)

	visited := make(container.Set[int64], cnt)
	err = db.Iterate(t.Context(), nil, func(ctx context.Context, repoUnit *repo_model.RepoUnit) error {
		assert.True(t, visited.Add(repoUnit.ID))
		has, err := db.ExistByID[repo_model.RepoUnit](ctx, repoUnit.ID) // the callback must be able to query on the same context
		assert.NoError(t, err)
		assert.True(t, has)
		return nil
	})
	assert.NoError(t, err)
	assert.Len(t, visited, int(cnt))
}
