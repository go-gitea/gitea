// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"sync"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertChannelOpen(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal(msg)
	default:
	}
}

func assertChannelClosed(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
	default:
		t.Fatal(msg)
	}
}

func TestTasksVersionWake_SignalClosesChannel(t *testing.T) {
	ch := TasksVersionWakeChannel()
	assertChannelOpen(t, ch, "channel should be open before signal")

	signalTaskVersionWake()
	assertChannelClosed(t, ch, "channel should be closed after signal")

	assertChannelOpen(t, TasksVersionWakeChannel(), "replacement channel should be open")
}

func TestIncreaseTaskVersion_SignalsWakeAfterCommit(t *testing.T) {
	for name, inTransaction := range map[string]bool{"Transaction": true, "NoTransaction": false} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())

			for version := range int64(2) {
				ch := TasksVersionWakeChannel()
				if inTransaction {
					require.NoError(t, db.WithTx(t.Context(), func(ctx context.Context) error {
						if err := IncreaseTaskVersion(ctx, 2, 1); err != nil {
							return err
						}
						assertChannelOpen(t, ch, "wake must wait for the outer transaction to commit")
						return nil
					}))
				} else {
					require.NoError(t, IncreaseTaskVersion(t.Context(), 2, 1))
				}
				assertChannelClosed(t, ch, "committing the version must wake waiters")
				for _, scope := range [][2]int64{{0, 0}, {2, 0}, {0, 1}} {
					actualVersion, err := GetTasksVersionByScope(t.Context(), scope[0], scope[1])
					require.NoError(t, err)
					assert.Equal(t, version+1, actualVersion)
				}
			}
		})
	}

	ch := TasksVersionWakeChannel()
	rollback := errors.New("rollback")
	assert.ErrorIs(t, db.WithTx(t.Context(), func(ctx context.Context) error {
		if err := IncreaseTaskVersion(ctx, 2, 1); err != nil {
			return err
		}
		assertChannelOpen(t, ch, "uncommitted versions must not wake waiters")
		return rollback
	}), rollback)
	assertChannelOpen(t, ch, "rolled back versions must not wake waiters")
}

func TestTasksVersionWake_ConcurrentSafe(t *testing.T) {
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() { _ = TasksVersionWakeChannel() })
		wg.Go(func() { signalTaskVersionWake() })
	}
	wg.Wait()

	assertChannelOpen(t, TasksVersionWakeChannel(), "a fresh subscription should be open until the next signal")
}
