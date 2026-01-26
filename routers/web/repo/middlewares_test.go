// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/services/contexttest"
	"code.gitea.io/gitea/services/gitdiff"

	"github.com/stretchr/testify/assert"
)

func TestDiffViewStyle(t *testing.T) {
	unittest.PrepareTestEnv(t)

	t.Run("AnonymousUser", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "/any")
		SetDiffViewStyle(ctx)
		assert.Equal(t, gitdiff.DiffStyleUnified, GetDiffViewStyle(ctx))

		ctx, _ = contexttest.MockContext(t, "/any?style=split")
		SetDiffViewStyle(ctx)
		assert.Equal(t, gitdiff.DiffStyleSplit, GetDiffViewStyle(ctx))

		ctx, _ = contexttest.MockContext(t, "/any")
		SetDiffViewStyle(ctx)
		assert.Equal(t, gitdiff.DiffStyleUnified, GetDiffViewStyle(ctx)) // at the moment, anonymous users don't have a saved preference
	})

	t.Run("SignedInUser", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "/any")
		contexttest.LoadUser(t, ctx, 2)
		SetDiffViewStyle(ctx)
		assert.Equal(t, gitdiff.DiffStyleUnified, GetDiffViewStyle(ctx))

		ctx, _ = contexttest.MockContext(t, "/any?style=split")
		contexttest.LoadUser(t, ctx, 2)
		SetDiffViewStyle(ctx)
		assert.Equal(t, gitdiff.DiffStyleSplit, GetDiffViewStyle(ctx))

		ctx, _ = contexttest.MockContext(t, "/any")
		contexttest.LoadUser(t, ctx, 2)
		SetDiffViewStyle(ctx)
		assert.Equal(t, gitdiff.DiffStyleSplit, GetDiffViewStyle(ctx))

		ctx, _ = contexttest.MockContext(t, "/any?style=unified")
		contexttest.LoadUser(t, ctx, 2)
		SetDiffViewStyle(ctx)
		assert.Equal(t, gitdiff.DiffStyleUnified, GetDiffViewStyle(ctx))

		ctx, _ = contexttest.MockContext(t, "/any")
		contexttest.LoadUser(t, ctx, 2)
		SetDiffViewStyle(ctx)
		assert.Equal(t, gitdiff.DiffStyleUnified, GetDiffViewStyle(ctx))
	})
}
