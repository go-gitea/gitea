// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
	assert.NoError(t, db.WithTx(func(ctx context.Context) error {
		assert.True(t, db.InTransaction(ctx))
		return nil
	}))
}
