// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"testing"

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
		_, err := InsertEvent(t.Context(), event)
		require.NoError(t, err)
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
