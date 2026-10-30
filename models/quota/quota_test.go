// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package quota

import (
	"context"
	"math"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { unittest.MainTest(m) }

func TestQuotaAdmission(t *testing.T) {
	for _, tc := range []struct {
		limit, used, growth int64
		allowed             bool
	}{
		{-1, math.MaxInt64, 0, true},
		{-1, 0, math.MaxInt64, true},
		{0, 0, 0, true},
		{0, 0, 1, false},
		{10, 9, 1, true},
		{10, 9, 2, false},
		{10, 20, 0, true},
		{10, 20, 1, false},
		{math.MaxInt64, math.MaxInt64, 1, false},
		{10, -1, 0, false},
		{10, 0, -1, false},
	} {
		err := Check("test", tc.limit, tc.used, tc.growth)
		assert.Equal(t, tc.allowed, err == nil, "%+v", tc)
	}
}

func TestQuotaPolicyAndReservation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()
	policy := &Policy{Subject: Repository, SubjectID: 1, GitLimit: new(int64(0)), LFSLimit: new(int64(-1))}
	require.NoError(t, SetPolicy(ctx, policy))
	loaded, err := GetPolicy(ctx, Repository, 1)
	require.NoError(t, err)
	require.NotNil(t, loaded.GitLimit)
	assert.EqualValues(t, 0, *loaded.GitLimit)
	assert.EqualValues(t, -1, *loaded.LFSLimit)
	loaded.GitLimit = nil
	require.NoError(t, SetPolicy(ctx, loaded))
	loaded, err = GetPolicy(ctx, Repository, 1)
	require.NoError(t, err)
	assert.Nil(t, loaded.GitLimit)
	policy.TotalLimit = new(int64(1))
	require.Error(t, SetPolicy(ctx, policy))
	op := &Operation{OwnerID: 1, RepoID: 1, Token: "first"}
	require.NoError(t, Acquire(ctx, op))
	assert.ErrorIs(t, Acquire(ctx, &Operation{OwnerID: 1, RepoID: 2, Token: "second"}), ErrBusy)
	assert.ErrorIs(t, Release(ctx, &Operation{OwnerID: 1, Token: "second"}), ErrUncertain)
	op.GitReserved = 1024
	require.NoError(t, Reserve(ctx, op))
	got, err := GetOperation(ctx, 1, "first")
	require.NoError(t, err)
	assert.EqualValues(t, 1024, got.GitReserved)
	require.NoError(t, AttachChild(ctx, op))
	assert.ErrorIs(t, Release(ctx, op), ErrUncertain)
	closed, err := BeginClose(ctx, op)
	require.NoError(t, err)
	assert.False(t, closed)
	require.NoError(t, DetachChild(ctx, op))
	closed, err = BeginClose(ctx, op)
	require.NoError(t, err)
	assert.True(t, closed)
	assert.ErrorIs(t, AttachChild(ctx, op), ErrUncertain)
	require.NoError(t, Release(ctx, op))
	assert.ErrorIs(t, Release(ctx, op), ErrUncertain)
	require.NoError(t, Acquire(ctx, &Operation{OwnerID: 1, RepoID: 2, Token: "second"}))
}

func TestQuotaPolicyLimits(t *testing.T) {
	defer test.MockVariableValue(&setting.Quota.DefaultRepoGitLimit, 123)()
	git, _, _ := (&Policy{Subject: Repository}).Limits()
	assert.EqualValues(t, 123, git)
	git, _, _ = (&Policy{Subject: Repository, GitLimit: new(int64(-1))}).Limits()
	assert.EqualValues(t, -1, git)
}

func TestQuotaUsageInvalidationPreservesCooldown(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	require.NoError(t, InvalidateUsage(ctx, 99999))
	_, found, err := GetUsage(ctx, 99999)
	require.NoError(t, err)
	require.False(t, found)
	require.NoError(t, StoreUsage(ctx, &Usage{RepoID: 1, OwnerID: 2, GitBytes: 10, LFSBytes: 20}))
	allowed, err := RequestGC(ctx, 1)
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, InvalidateUsage(ctx, 1))
	usage, found, err := GetUsage(ctx, 1)
	require.NoError(t, err)
	require.True(t, found)
	require.Zero(t, usage.Updated)
	require.EqualValues(t, 10, usage.GitBytes)
	require.EqualValues(t, 20, usage.LFSBytes)
	allowed, err = RequestGC(ctx, 1)
	require.NoError(t, err)
	require.False(t, allowed)
	_, err = db.GetEngine(ctx).ID(1).NoAutoTime().Cols("gc_requested").Update(&Usage{GCRequested: timeutil.TimeStampNow() - 3601})
	require.NoError(t, err)
	require.NoError(t, StoreUsage(ctx, &Usage{RepoID: 1, OwnerID: 2, GitBytes: 10}))
	allowed, err = RequestGC(ctx, 1)
	require.NoError(t, err)
	require.True(t, allowed)
}
