// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package process

import (
	"context"
	"testing"

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

	ctx, cancel := context.WithCancel(t.Context())
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

	ctx, _, finished := pm.AddContext(t.Context(), "foo")
	defer finished()

	pm.Cancel(GetPID(ctx))

	select {
	case <-ctx.Done():
	default:
		assert.FailNow(t, "Cancel should cancel the provided context")
	}
	finished()

	ctx, cancel, finished := pm.AddContext(t.Context(), "foo")
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

	ctx, cancel := context.WithCancel(t.Context())
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
