// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"testing"
	"time"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindEventsScopeFilters(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	events := []*Event{
		{Action: UserCreate, ScopeType: ScopeUser, ScopeID: 5, Origin: OriginUI, TimestampUnix: timeutil.TimeStamp(1)},
		{Action: RepositoryCreate, ScopeType: ScopeRepository, ScopeID: 5, Origin: OriginAPI, TimestampUnix: timeutil.TimeStamp(1)},
		{Action: RepositoryCreate, ScopeType: ScopeRepository, ScopeID: 6, Origin: OriginCLI, TimestampUnix: timeutil.TimeStamp(1)},
		{Action: RepositoryCreate, ScopeType: ScopeRepository, ScopeID: 7, Origin: OriginSystem, TimestampUnix: timeutil.TimeStamp(1)},
	}
	for _, event := range events {
		require.NoError(t, InsertEvent(t.Context(), event))
	}

	byType, _, err := FindEvents(t.Context(), &EventSearchOptions{ScopeType: ScopeRepository})
	require.NoError(t, err)
	assert.Len(t, byType, 3)

	byID, _, err := FindEvents(t.Context(), &EventSearchOptions{ScopeID: 5})
	require.NoError(t, err)
	assert.Len(t, byID, 2)

	byScope, _, err := FindEvents(t.Context(), &EventSearchOptions{ScopeType: ScopeRepository, ScopeID: 5})
	require.NoError(t, err)
	assert.Len(t, byScope, 1)

	byOrigin, _, err := FindEvents(t.Context(), &EventSearchOptions{Origin: OriginAPI})
	require.NoError(t, err)
	assert.Len(t, byOrigin, 1)

	bySystemOrigin, _, err := FindEvents(t.Context(), &EventSearchOptions{Origin: OriginSystem})
	require.NoError(t, err)
	assert.Len(t, bySystemOrigin, 1)
}

// Filtering for an admin must surface what they did while impersonating someone.
func TestFindEventsActorFilterIncludesImpersonations(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	events := []*Event{
		{Action: UserPassword, ActorID: 10, ScopeType: ScopeUser, ScopeID: 10, TimestampUnix: timeutil.TimeStamp(1)},
		{Action: UserPassword, ActorID: 11, ImpersonatorID: 10, ScopeType: ScopeUser, ScopeID: 11, TimestampUnix: timeutil.TimeStamp(2)},
		{Action: UserPassword, ActorID: 12, ScopeType: ScopeUser, ScopeID: 12, TimestampUnix: timeutil.TimeStamp(3)},
	}
	for _, event := range events {
		require.NoError(t, InsertEvent(t.Context(), event))
	}

	byAdmin, _, err := FindEvents(t.Context(), &EventSearchOptions{ActorID: 10})
	require.NoError(t, err)
	assert.Len(t, byAdmin, 2)

	byImpersonated, _, err := FindEvents(t.Context(), &EventSearchOptions{ActorID: 11})
	require.NoError(t, err)
	assert.Len(t, byImpersonated, 1)
}

func TestDeleteOldEvents(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	now := time.Now()
	old := &Event{Action: UserCreate, ScopeType: ScopeUser, ScopeID: 1, TimestampUnix: timeutil.TimeStamp(now.Add(-48 * time.Hour).Unix())}
	recent := &Event{Action: UserCreate, ScopeType: ScopeUser, ScopeID: 2, TimestampUnix: timeutil.TimeStamp(now.Unix())}
	require.NoError(t, InsertEvent(t.Context(), old))
	require.NoError(t, InsertEvent(t.Context(), recent))

	require.NoError(t, DeleteOldEvents(t.Context(), 0)) // keeps everything
	_, count, err := FindEvents(t.Context(), &EventSearchOptions{})
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)

	require.NoError(t, DeleteOldEvents(t.Context(), 24*time.Hour))
	remaining, _, err := FindEvents(t.Context(), &EventSearchOptions{})
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, recent.ID, remaining[0].ID)
}
