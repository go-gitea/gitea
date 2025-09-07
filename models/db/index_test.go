// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

type TestIndex db.ResourceIndex

func getCurrentResourceIndex(ctx context.Context, tableName string, groupID int64) (int64, error) {
	e := db.GetEngine(ctx)
	var idx int64
	has, err := e.SQL(fmt.Sprintf("SELECT max_index FROM %s WHERE group_id=?", tableName), groupID).Get(&idx)
	if err != nil {
		return 0, err
	}
	if !has {
		return 0, errors.New("no record")
	}
	return idx, nil
}

func TestSyncMaxResourceIndex(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	xe := unittest.GetXORMEngine()
	assert.NoError(t, xe.Sync(&TestIndex{}))

	err := db.SyncMaxResourceIndex(t.Context(), "test_index", 10, 51)
	assert.NoError(t, err)

	// sync new max index
	maxIndex, err := getCurrentResourceIndex(t.Context(), "test_index", 10)
	assert.NoError(t, err)
	assert.EqualValues(t, 51, maxIndex)

	// smaller index doesn't change
	err = db.SyncMaxResourceIndex(t.Context(), "test_index", 10, 30)
	assert.NoError(t, err)
	maxIndex, err = getCurrentResourceIndex(t.Context(), "test_index", 10)
	assert.NoError(t, err)
	assert.EqualValues(t, 51, maxIndex)

	// larger index changes
	err = db.SyncMaxResourceIndex(t.Context(), "test_index", 10, 62)
	assert.NoError(t, err)
	maxIndex, err = getCurrentResourceIndex(t.Context(), "test_index", 10)
	assert.NoError(t, err)
	assert.EqualValues(t, 62, maxIndex)

	// commit transaction
	err = db.WithTx(t.Context(), func(ctx context.Context) error {
		err = db.SyncMaxResourceIndex(ctx, "test_index", 10, 73)
		assert.NoError(t, err)
		maxIndex, err = getCurrentResourceIndex(ctx, "test_index", 10)
		assert.NoError(t, err)
		assert.EqualValues(t, 73, maxIndex)
		return nil
	})
	assert.NoError(t, err)
	maxIndex, err = getCurrentResourceIndex(t.Context(), "test_index", 10)
	assert.NoError(t, err)
	assert.EqualValues(t, 73, maxIndex)

	// rollback transaction
	err = db.WithTx(t.Context(), func(ctx context.Context) error {
		err = db.SyncMaxResourceIndex(ctx, "test_index", 10, 84)
		maxIndex, err = getCurrentResourceIndex(ctx, "test_index", 10)
		assert.NoError(t, err)
		assert.EqualValues(t, 84, maxIndex)
		return errors.New("test rollback")
	})
	assert.Error(t, err)
	maxIndex, err = getCurrentResourceIndex(t.Context(), "test_index", 10)
	assert.NoError(t, err)
	assert.EqualValues(t, 73, maxIndex) // the max index doesn't change because the transaction was rolled back
}

func TestGetNextResourceIndex(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	xe := unittest.GetXORMEngine()
	assert.NoError(t, xe.Sync(&TestIndex{}))

	// create a new record
	maxIndex, err := db.GetNextResourceIndex(t.Context(), "test_index", 20)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, maxIndex)

	// increase the existing record
	maxIndex, err = db.GetNextResourceIndex(t.Context(), "test_index", 20)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, maxIndex)

	// commit transaction
	err = db.WithTx(t.Context(), func(ctx context.Context) error {
		maxIndex, err = db.GetNextResourceIndex(ctx, "test_index", 20)
		assert.NoError(t, err)
		assert.EqualValues(t, 3, maxIndex)
		return nil
	})
	assert.NoError(t, err)
	maxIndex, err = getCurrentResourceIndex(t.Context(), "test_index", 20)
	assert.NoError(t, err)
	assert.EqualValues(t, 3, maxIndex)

	// rollback transaction
	err = db.WithTx(t.Context(), func(ctx context.Context) error {
		maxIndex, err = db.GetNextResourceIndex(ctx, "test_index", 20)
		assert.NoError(t, err)
		assert.EqualValues(t, 4, maxIndex)
		return errors.New("test rollback")
	})
	assert.Error(t, err)
	maxIndex, err = getCurrentResourceIndex(t.Context(), "test_index", 20)
	assert.NoError(t, err)
	assert.EqualValues(t, 3, maxIndex) // the max index doesn't change because the transaction was rolled back
}
