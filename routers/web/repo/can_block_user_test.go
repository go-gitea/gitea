// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCachedCanBlockUser(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1/pulls/1")
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	blockee := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	ctx.Doer = doer

	canBlock := newCachedCanBlockUser(ctx)
	assert.True(t, canBlock(doer, blockee))
	assert.False(t, canBlock(doer, doer)) // never block oneself

	// a block created after the first call isn't visible through the cache,
	// which proves that the database isn't queried again for the same pair
	require.NoError(t, db.Insert(t.Context(), &user_model.Blocking{BlockerID: doer.ID, BlockeeID: blockee.ID}))
	assert.True(t, canBlock(doer, blockee))
	assert.False(t, newCachedCanBlockUser(ctx)(doer, blockee))
}
