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

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestIncreaseTaskVersionWakesItsScopesAfterCommit(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	scopes := [][2]int64{{0, 0}, {2, 0}, {0, 1}}
	wakeChannels := func() (chans []<-chan struct{}) {
		for _, scope := range scopes {
			chans = append(chans, TasksVersionWakeChannel(scope[0], scope[1]))
		}
		return chans
	}

	chans := wakeChannels()
	other := TasksVersionWakeChannel(3, 0)
	require.NoError(t, db.WithTx(t.Context(), func(ctx context.Context) error {
		require.NoError(t, IncreaseTaskVersion(ctx, 2, 1))
		for _, ch := range chans {
			assert.False(t, isClosed(ch))
		}
		return nil
	}))
	for _, ch := range chans {
		assert.True(t, isClosed(ch))
	}
	assert.False(t, isClosed(other))

	chans = wakeChannels()
	rollback := errors.New("rollback")
	assert.ErrorIs(t, db.WithTx(t.Context(), func(ctx context.Context) error {
		require.NoError(t, IncreaseTaskVersion(ctx, 2, 1))
		return rollback
	}), rollback)
	for _, ch := range chans {
		assert.False(t, isClosed(ch))
	}

	for _, scope := range scopes {
		version, err := GetTasksVersionByScope(t.Context(), scope[0], scope[1])
		require.NoError(t, err)
		assert.EqualValues(t, 1, version)
	}
}

func TestWatchTasksVersionWakesScopesChangedByAnotherProcess(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&tasksVersionWatchInterval, 10*time.Millisecond)()
	changedRepoID := time.Now().UnixNano()
	otherRepoID := changedRepoID + 1
	require.NoError(t, IncreaseTaskVersion(t.Context(), 0, changedRepoID))
	require.NoError(t, IncreaseTaskVersion(t.Context(), 0, otherRepoID))
	defer WatchTasksVersion()()

	waitClosed := func(ch <-chan struct{}) {
		t.Helper()
		select {
		case <-ch:
		case <-time.After(5 * time.Second):
			t.Fatal("waiter not woken")
		}
	}
	changed, other := TasksVersionWakeChannel(0, changedRepoID), TasksVersionWakeChannel(0, otherRepoID)
	waitClosed(changed)
	waitClosed(other)

	changed, other = TasksVersionWakeChannel(0, changedRepoID), TasksVersionWakeChannel(0, otherRepoID)
	_, err := db.GetEngine(t.Context()).Exec("UPDATE action_tasks_version SET version = version + 1 WHERE owner_id = 0 AND repo_id = ?", changedRepoID)
	require.NoError(t, err)
	waitClosed(changed)
	assert.False(t, isClosed(other))
}
