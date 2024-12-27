// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db_test

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

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

func TestContextSafety(t *testing.T) {
	type TestModel1 struct {
		ID int64
	}
	type TestModel2 struct {
		ID int64
	}
	assert.NoError(t, unittest.GetXORMEngine().Sync(&TestModel1{}, &TestModel2{}))
	assert.NoError(t, db.TruncateBeans(db.DefaultContext, &TestModel1{}, &TestModel2{}))
	testCount := 10
	for i := 1; i <= testCount; i++ {
		assert.NoError(t, db.Insert(db.DefaultContext, &TestModel1{ID: int64(i)}))
		assert.NoError(t, db.Insert(db.DefaultContext, &TestModel2{ID: int64(-i)}))
	}

	actualCount := 0
	// here: db.GetEngine(db.DefaultContext) is a new *Session created from *Engine
	_ = db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		_ = db.GetEngine(ctx).Iterate(&TestModel1{}, func(i int, bean any) error {
			// here: db.GetEngine(ctx) is always the unclosed "Iterate" *Session with autoResetStatement=false,
			// and the internal states (including "cond" and others) are always there and not be reset in this callback.
			m1 := bean.(*TestModel1)
			assert.EqualValues(t, i+1, m1.ID)

			// here: XORM bug, it fails because the SQL becomes "WHERE id=-1", "WHERE id=-1 AND id=-2", "WHERE id=-1 AND id=-2 AND id=-3" ...
			// and it conflicts with the "Iterate"'s internal states.
			// has, err := db.GetEngine(ctx).Get(&TestModel2{ID: -m1.ID})

			actualCount++
			return nil
		})
		return nil
	})
	assert.EqualValues(t, testCount, actualCount)

	// deny the bad usages
	assert.PanicsWithError(t, "using database context in an iterator would cause corrupted results", func() {
		_ = unittest.GetXORMEngine().Iterate(&TestModel1{}, func(i int, bean any) error {
			_ = db.GetEngine(db.DefaultContext)
			return nil
		})
	})
}
