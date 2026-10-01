// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type gracefulManagerContextKey struct{}

func TestGracefulManagerContextIgnoresParentCancellation(t *testing.T) {
	key := gracefulManagerContextKey{}
	parentCtx, cancelParent := context.WithCancel(context.WithValue(t.Context(), key, "value"))
	managerCtx, cancelManager := newGracefulManagerContext(parentCtx)
	t.Cleanup(cancelManager)

	cancelParent()

	assert.NoError(t, managerCtx.Err())
	assert.Equal(t, "value", managerCtx.Value(key))

	cancelManager()

	assert.ErrorIs(t, managerCtx.Err(), context.Canceled)
}
