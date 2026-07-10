// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "gitea.dev/models/actions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTryPickTaskThrottled(t *testing.T) {
	sem := taskPickLimiter()

	// Saturate every assignment slot so the next attempt must be throttled.
	for range cap(sem) {
		sem <- struct{}{}
	}
	defer func() {
		for range cap(sem) {
			<-sem
		}
	}()

	// No DB access happens on the throttled path, so this is safe without fixtures.
	task, ok, throttled, err := TryPickTask(t.Context(), &actions_model.ActionRunner{})
	require.NoError(t, err)
	assert.Nil(t, task)
	assert.False(t, ok)
	assert.True(t, throttled)
}
