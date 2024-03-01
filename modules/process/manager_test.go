// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package process

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetManager(t *testing.T) {
	go func() {
		// test race protection
		_ = GetManager()
	}()
	pm := GetManager()
	assert.NotNil(t, pm)
}

func TestManager_AddContext(t *testing.T) {
	pm := Manager{processMap: make(map[IDType]*process), next: 1}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p1Ctx, _, finished := pm.AddContext(ctx, "foo")
	defer finished()
	assert.NotEmpty(t, GetContext(p1Ctx).GetPID(), "expected to get non-empty pid")

	p2Ctx, _, finished := pm.AddContext(p1Ctx, "bar")
	defer finished()

	assert.NotEmpty(t, GetContext(p2Ctx).GetPID(), "expected to get non-empty pid")

	assert.NotEqual(t, GetContext(p1Ctx).GetPID(), GetContext(p2Ctx).GetPID(), "expected to get different pids %s == %s", GetContext(p2Ctx).GetPID(), GetContext(p1Ctx).GetPID())
	assert.Equal(t, GetContext(p1Ctx).GetPID(), GetContext(p2Ctx).GetParent().GetPID(), "expected to get pid %s got %s", GetContext(p1Ctx).GetPID(), GetContext(p2Ctx).GetParent().GetPID())
}

func TestManager_Cancel(t *testing.T) {
	pm := Manager{processMap: make(map[IDType]*process), next: 1}

	ctx, _, finished := pm.AddContext(context.Background(), "foo")
	defer finished()

	pm.Cancel(GetPID(ctx))

	select {
	case <-ctx.Done():
	default:
		assert.FailNow(t, "Cancel should cancel the provided context")
	}
	finished()

	ctx, cancel, finished := pm.AddContext(context.Background(), "foo")
	defer finished()

	cancel()

	select {
	case <-ctx.Done():
	default:
		assert.FailNow(t, "Cancel should cancel the provided context")
	}
	finished()
}

func TestManager_Remove(t *testing.T) {
	pm := Manager{processMap: make(map[IDType]*process), next: 1}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p1Ctx, _, finished := pm.AddContext(ctx, "foo")
	defer finished()
	assert.NotEmpty(t, GetContext(p1Ctx).GetPID(), "expected to have non-empty PID")

	p2Ctx, _, finished := pm.AddContext(p1Ctx, "bar")
	defer finished()

	assert.NotEqual(t, GetContext(p1Ctx).GetPID(), GetContext(p2Ctx).GetPID(), "expected to get different pids got %s == %s", GetContext(p2Ctx).GetPID(), GetContext(p1Ctx).GetPID())

	finished()

	_, exists := pm.processMap[GetPID(p2Ctx)]
	assert.False(t, exists, "PID %d is in the list but shouldn't", GetPID(p2Ctx))
}

func TestExecTimeoutNever(t *testing.T) {
	// TODO Investigate how to improve the time elapsed per round.
	maxLoops := 10
	for i := 1; i < maxLoops; i++ {
		_, stderr, err := GetManager().ExecTimeout(5*time.Second, "ExecTimeout", "git", "--version")
		if err != nil {
			t.Fatalf("git --version: %v(%s)", err, stderr)
		}
	}
}

func TestExecTimeoutAlways(t *testing.T) {
	maxLoops := 100
	for i := 1; i < maxLoops; i++ {
		_, stderr, err := GetManager().ExecTimeout(100*time.Microsecond, "ExecTimeout", "sleep", "5")
		// TODO Simplify logging and errors to get precise error type. E.g. checking "if err != context.DeadlineExceeded".
		if err == nil {
			t.Fatalf("sleep 5 secs: %v(%s)", err, stderr)
		}
	}
}
