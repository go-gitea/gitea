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

	for _, event := range []*Event{
		{Action: UserCreate, ScopeType: ScopeUser, ScopeID: 5, Origin: OriginUI, TimestampUnix: timeutil.TimeStamp(1)},
		{Action: RepositoryCreate, ScopeType: ScopeRepository, ScopeID: 5, Origin: OriginAPI, TimestampUnix: timeutil.TimeStamp(1)},
		{Action: RepositoryCreate, ScopeType: ScopeRepository, ScopeID: 6, Origin: OriginCLI, TimestampUnix: timeutil.TimeStamp(1)},
		{Action: RepositoryCreate, ScopeType: ScopeRepository, ScopeID: 7, Origin: OriginSystem, TimestampUnix: timeutil.TimeStamp(1)},
	} {
		require.NoError(t, InsertEvent(t.Context(), event))
	}

	for _, test := range []struct {
		options EventSearchOptions
		count   int
	}{
		{EventSearchOptions{ScopeType: ScopeRepository}, 3},
		{EventSearchOptions{ScopeID: 5}, 2},
		{EventSearchOptions{ScopeType: ScopeRepository, ScopeID: 5}, 1},
		{EventSearchOptions{Origin: OriginAPI}, 1},
		{EventSearchOptions{Origin: OriginSystem}, 1},
	} {
		events, _, err := FindEvents(t.Context(), &test.options)
		require.NoError(t, err)
		assert.Len(t, events, test.count)
	}
}

func TestFindEventsActionPrefixFilter(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	for _, action := range []Action{UserImpersonation, UserImpersonationExit, UserCreate} {
		require.NoError(t, InsertEvent(t.Context(), &Event{Action: action, ScopeType: ScopeUser, ScopeID: 1, TimestampUnix: timeutil.TimeStamp(1)}))
	}

	events, _, err := FindEvents(t.Context(), &EventSearchOptions{ActionPrefix: "user:impersonation"})
	require.NoError(t, err)
	assert.Len(t, events, 2)

	exact, _, err := FindEvents(t.Context(), &EventSearchOptions{Action: UserImpersonationExit})
	require.NoError(t, err)
	assert.Len(t, exact, 1)
	assert.Equal(t, UserImpersonationExit, exact[0].Action)
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
