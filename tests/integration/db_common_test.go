// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestInsertOnConflictDoNothing(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	ctx := db.DefaultContext
	e := db.GetEngine(ctx)
	t.Run("NoUnique", func(t *testing.T) {
		type NoUniques struct {
			ID   int64 `xorm:"pk autoincr"`
			Data string
		}
		_ = e.Sync2(&NoUniques{})

		has, err := db.InsertOnConflictDoNothing(ctx, &NoUniques{Data: "shouldErr"})
		assert.Error(t, err)
		assert.False(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &NoUniques{Data: ""})
		assert.Error(t, err)
		assert.False(t, has)
	})

	t.Run("OneUnique", func(t *testing.T) {
		type OneUnique struct {
			ID   int64  `xorm:"pk autoincr"`
			Data string `xorm:"UNIQUE NOT NULL"`
		}

		_ = e.Sync2(&OneUnique{})
		_, _ = e.Exec("DELETE FROM one_unique")

		has, err := db.InsertOnConflictDoNothing(ctx, &OneUnique{})
		assert.Error(t, err)
		assert.False(t, has)

		toInsert := &OneUnique{Data: "test"}

		has, err = db.InsertOnConflictDoNothing(ctx, toInsert)
		assert.NoError(t, err)
		assert.True(t, has)
		assert.NotEqual(t, 0, toInsert.ID)

		has, err = db.InsertOnConflictDoNothing(ctx, &OneUnique{Data: "test2"})
		assert.NoError(t, err)
		assert.True(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &OneUnique{Data: "test"})
		assert.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("MultiUnique", func(t *testing.T) {
		type MultiUnique struct {
			ID        int64 `xorm:"pk autoincr"`
			NotUnique string
			Data1     string `xorm:"UNIQUE(s) NOT NULL"`
			Data2     string `xorm:"UNIQUE(s) NOT NULL"`
		}

		_ = e.Sync2(&MultiUnique{})
		_, _ = e.Exec("DELETE FROM multi_unique")

		has, err := db.InsertOnConflictDoNothing(ctx, &MultiUnique{})
		assert.Error(t, err)
		assert.False(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &MultiUnique{Data1: "test", NotUnique: "t1"})
		assert.NoError(t, err)
		assert.True(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &MultiUnique{Data2: "test2", NotUnique: "t1"})
		assert.NoError(t, err)
		assert.True(t, has)
		has, err = db.InsertOnConflictDoNothing(ctx, &MultiUnique{Data2: "test2", NotUnique: "t2"})
		assert.NoError(t, err)
		assert.False(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &MultiUnique{Data1: "test", NotUnique: "t2"})
		assert.NoError(t, err)
		assert.False(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &MultiUnique{Data1: "test", Data2: "test2", NotUnique: "t1"})
		assert.NoError(t, err)
		assert.True(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &MultiUnique{Data1: "test", Data2: "test2", NotUnique: "t2"})
		assert.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("MultiMultiUnique", func(t *testing.T) {
		type MultiMultiUnique struct {
			ID    int64  `xorm:"pk autoincr"`
			Data0 string `xorm:"UNIQUE NOT NULL"`
			Data1 string `xorm:"UNIQUE(s) NOT NULL"`
			Data2 string `xorm:"UNIQUE(s) NOT NULL"`
		}

		_ = e.Sync2(&MultiMultiUnique{})
		_, _ = e.Exec("DELETE FROM multi_multi_unique")

		has, err := db.InsertOnConflictDoNothing(ctx, &MultiMultiUnique{})
		assert.Error(t, err)
		assert.False(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &MultiMultiUnique{Data1: "test", Data0: "t1"})
		assert.NoError(t, err)
		assert.True(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &MultiMultiUnique{Data2: "test2", Data0: "t1"})
		assert.NoError(t, err)
		assert.False(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &MultiMultiUnique{Data2: "test2", Data0: "t2"})
		assert.NoError(t, err)
		assert.True(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &MultiMultiUnique{Data2: "test2", Data0: "t2"})
		assert.NoError(t, err)
		assert.False(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &MultiMultiUnique{Data1: "test", Data0: "t2"})
		assert.NoError(t, err)
		assert.False(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &MultiMultiUnique{Data1: "test", Data2: "test2", Data0: "t3"})
		assert.NoError(t, err)
		assert.True(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &MultiMultiUnique{Data1: "test", Data2: "test2", Data0: "t2"})
		assert.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("NoPK", func(t *testing.T) {
		type NoPrimaryKey struct {
			NotID   int64
			Uniqued string `xorm:"UNIQUE"`
		}

		err := e.Sync2(&NoPrimaryKey{})
		assert.NoError(t, err)
		_, _ = e.Exec("DELETE FROM no_primary_key")

		empty := &NoPrimaryKey{}
		has, err := db.InsertOnConflictDoNothing(ctx, empty)
		assert.Error(t, err)
		assert.False(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &NoPrimaryKey{Uniqued: "1"})
		assert.NoError(t, err)
		assert.True(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &NoPrimaryKey{NotID: 1})
		assert.NoError(t, err)
		assert.True(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &NoPrimaryKey{NotID: 2})
		assert.NoError(t, err)
		assert.False(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &NoPrimaryKey{NotID: 2, Uniqued: "2"})
		assert.NoError(t, err)
		assert.True(t, has)

		has, err = db.InsertOnConflictDoNothing(ctx, &NoPrimaryKey{NotID: 1, Uniqued: "2"})
		assert.NoError(t, err)
		assert.False(t, has)
	})
}
