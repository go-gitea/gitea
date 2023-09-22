// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db_test

import (
	"context"
	"testing"

	"code.gitea.io/gitea/internal/models/db"
	"code.gitea.io/gitea/internal/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestInTransaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.False(t, db.InTransaction(db.DefaultContext))
	assert.NoError(t, db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		assert.True(t, db.InTransaction(ctx))
		return nil
	}))

	ctx, committer, err := db.TxContext(db.DefaultContext)
	assert.NoError(t, err)
	defer committer.Close()
	assert.True(t, db.InTransaction(ctx))
	assert.NoError(t, db.WithTx(ctx, func(ctx context.Context) error {
		assert.True(t, db.InTransaction(ctx))
		return nil
	}))
}

func TestTxContext(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	{ // create new transaction
		ctx, committer, err := db.TxContext(db.DefaultContext)
		assert.NoError(t, err)
		assert.True(t, db.InTransaction(ctx))
		assert.NoError(t, committer.Commit())
	}

	{ // reuse the transaction created by TxContext and commit it
		ctx, committer, err := db.TxContext(db.DefaultContext)
		engine := db.GetEngine(ctx)
		assert.NoError(t, err)
		assert.True(t, db.InTransaction(ctx))
		{
			ctx, committer, err := db.TxContext(ctx)
			assert.NoError(t, err)
			assert.True(t, db.InTransaction(ctx))
			assert.Equal(t, engine, db.GetEngine(ctx))
			assert.NoError(t, committer.Commit())
		}
		assert.NoError(t, committer.Commit())
	}

	{ // reuse the transaction created by TxContext and close it
		ctx, committer, err := db.TxContext(db.DefaultContext)
		engine := db.GetEngine(ctx)
		assert.NoError(t, err)
		assert.True(t, db.InTransaction(ctx))
		{
			ctx, committer, err := db.TxContext(ctx)
			assert.NoError(t, err)
			assert.True(t, db.InTransaction(ctx))
			assert.Equal(t, engine, db.GetEngine(ctx))
			assert.NoError(t, committer.Close())
		}
		assert.NoError(t, committer.Close())
	}

	{ // reuse the transaction created by WithTx
		assert.NoError(t, db.WithTx(db.DefaultContext, func(ctx context.Context) error {
			assert.True(t, db.InTransaction(ctx))
			{
				ctx, committer, err := db.TxContext(ctx)
				assert.NoError(t, err)
				assert.True(t, db.InTransaction(ctx))
				assert.NoError(t, committer.Commit())
			}
			return nil
		}))
	}
}
