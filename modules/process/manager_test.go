// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
	pm := Manager{processes: make(map[IDType]*Process), next: 1}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p1Ctx, _, remove := pm.AddContext(ctx, "foo")
	defer remove()
	assert.Equal(t, int64(1), GetContext(p1Ctx).GetPID(), "expected to get pid 1 got %d", GetContext(p1Ctx).GetPID())

	p2Ctx, _, remove := pm.AddContext(p1Ctx, "bar")
	defer remove()

	assert.Equal(t, int64(2), GetContext(p2Ctx).GetPID(), "expected to get pid 2 got %d", GetContext(p2Ctx).GetPID())
	assert.Equal(t, int64(1), GetContext(p2Ctx).GetParent().GetPID(), "expected to get pid 1 got %d", GetContext(p2Ctx).GetParent().GetPID())
}

func TestManager_Cancel(t *testing.T) {
	pm := Manager{processes: make(map[IDType]*Process), next: 1}

	ctx, _, remove := pm.AddContext(context.Background(), "foo")
	defer remove()

	pm.Cancel(GetPID(ctx))

	select {
	case <-ctx.Done():
	default:
		assert.Fail(t, "Cancel should cancel the provided context")
	}
	remove()

	ctx, cancel, remove := pm.AddContext(context.Background(), "foo")
	defer remove()

	cancel()

	select {
	case <-ctx.Done():
	default:
		assert.Fail(t, "Cancel should cancel the provided context")
	}
	remove()
}

func TestManager_Remove(t *testing.T) {
	pm := Manager{processes: make(map[IDType]*Process), next: 1}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p1Ctx, _, remove := pm.AddContext(ctx, "foo")
	defer remove()
	assert.Equal(t, int64(1), GetContext(p1Ctx).GetPID(), "expected to get pid 1 got %d", GetContext(p1Ctx).GetPID())

	p2Ctx, _, remove := pm.AddContext(p1Ctx, "bar")
	defer remove()

	assert.Equal(t, int64(2), GetContext(p2Ctx).GetPID(), "expected to get pid 2 got %d", GetContext(p2Ctx).GetPID())

	pm.Remove(GetPID(p2Ctx))

	_, exists := pm.processes[GetPID(p2Ctx)]
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
