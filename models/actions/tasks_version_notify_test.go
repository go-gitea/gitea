// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTasksVersionWake(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&tasksVersionWatchInterval, 10*time.Millisecond)()
	isClosed := func(ch <-chan struct{}) bool {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}
	ownerID := time.Now().UnixNano()
	repoID := ownerID + 1
	scopeChannels := func() []<-chan struct{} {
		return []<-chan struct{}{TasksVersionWakeChannel(0, 0), TasksVersionWakeChannel(ownerID, 0), TasksVersionWakeChannel(0, repoID)}
	}

	chans, other := scopeChannels(), TasksVersionWakeChannel(ownerID+2, 0)
	require.NoError(t, db.WithTx(t.Context(), func(ctx context.Context) error {
		require.NoError(t, IncreaseTaskVersion(ctx, ownerID, repoID))
		assert.False(t, isClosed(chans[0]), "woken before commit")
		return nil
	}))
	for _, ch := range chans {
		assert.True(t, isClosed(ch), "not woken by the commit")
	}
	assert.False(t, isClosed(other), "woken by another scope's commit")

	chans = scopeChannels()
	rollback := errors.New("rollback")
	assert.ErrorIs(t, db.WithTx(t.Context(), func(ctx context.Context) error {
		require.NoError(t, IncreaseTaskVersion(ctx, ownerID, repoID))
		return rollback
	}), rollback)
	for _, ch := range chans {
		assert.False(t, isClosed(ch), "woken by a rollback")
	}

	defer WatchTasksVersion()()
	for _, ch := range append(chans[1:], other) {
		<-ch
	}
	repo, owner := TasksVersionWakeChannel(0, repoID), TasksVersionWakeChannel(ownerID, 0)
	_, err := db.GetEngine(t.Context()).Exec("UPDATE action_tasks_version SET version = version + 1 WHERE owner_id = 0 AND repo_id = ?", repoID)
	require.NoError(t, err)
	<-repo
	assert.False(t, isClosed(owner), "woken by another process changing another scope")
}
