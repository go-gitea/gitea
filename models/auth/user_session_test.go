// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateUserSession(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	sess := &auth_model.UserSession{
		ID:          "test-session-create",
		UserID:      1,
		LoginIP:     "192.168.1.1",
		LastIP:      "192.168.1.1",
		UserAgent:   "Mozilla/5.0 Test",
		LoginMethod: "form",
	}
	require.NoError(t, auth_model.CreateUserSession(t.Context(), sess))
	unittest.AssertExistsAndLoadBean(t, &auth_model.UserSession{ID: "test-session-create"})
}

func TestGetUserSessionByID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	sess := &auth_model.UserSession{
		ID:          "test-session-get",
		UserID:      2,
		LoginIP:     "10.0.0.1",
		LastIP:      "10.0.0.1",
		UserAgent:   "TestAgent",
		LoginMethod: "oauth2",
	}
	require.NoError(t, auth_model.CreateUserSession(t.Context(), sess))

	got, err := auth_model.GetUserSessionByID(t.Context(), "test-session-get")
	require.NoError(t, err)
	assert.Equal(t, int64(2), got.UserID)
	assert.Equal(t, "10.0.0.1", got.LoginIP)
	assert.Equal(t, "TestAgent", got.UserAgent)

	_, err = auth_model.GetUserSessionByID(t.Context(), "nonexistent")
	assert.True(t, auth_model.IsErrUserSessionNotExist(err))
}

func TestGetUserSessionsByUserID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	for _, id := range []string{"sess-list-1", "sess-list-2", "sess-list-3"} {
		require.NoError(t, auth_model.CreateUserSession(t.Context(), &auth_model.UserSession{
			ID:      id,
			UserID:  5,
			LoginIP: "127.0.0.1",
			LastIP:  "127.0.0.1",
		}))
	}

	sessions, err := auth_model.GetUserSessionsByUserID(t.Context(), 5)
	require.NoError(t, err)
	assert.Len(t, sessions, 3)

	sessions, err = auth_model.GetUserSessionsByUserID(t.Context(), 99999)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestInvalidateUserSession(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	require.NoError(t, auth_model.CreateUserSession(t.Context(), &auth_model.UserSession{
		ID:     "sess-invalidate",
		UserID: 1,
	}))

	require.NoError(t, auth_model.InvalidateUserSession(t.Context(), "sess-invalidate"))

	got, err := auth_model.GetUserSessionByID(t.Context(), "sess-invalidate")
	require.NoError(t, err)
	assert.NotZero(t, got.LogoutUnix, "LogoutUnix should be set after invalidation")
}

func TestInvalidateAllUserSessions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	for _, id := range []string{"sess-all-1", "sess-all-2", "sess-all-3"} {
		require.NoError(t, auth_model.CreateUserSession(t.Context(), &auth_model.UserSession{
			ID:     id,
			UserID: 3,
		}))
	}

	// Invalidate all except sess-all-2
	require.NoError(t, auth_model.InvalidateAllUserSessions(t.Context(), 3, "sess-all-2"))

	kept, err := auth_model.GetUserSessionByID(t.Context(), "sess-all-2")
	require.NoError(t, err)
	assert.Zero(t, kept.LogoutUnix, "excluded session should not be invalidated")

	for _, id := range []string{"sess-all-1", "sess-all-3"} {
		got, err := auth_model.GetUserSessionByID(t.Context(), id)
		require.NoError(t, err)
		assert.NotZero(t, got.LogoutUnix, "session %s should be invalidated", id)
	}
}

func TestUpdateSessionActivity(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	require.NoError(t, auth_model.CreateUserSession(t.Context(), &auth_model.UserSession{
		ID:     "sess-activity",
		UserID: 1,
		LastIP: "10.0.0.1",
	}))

	// Update with same IP — only LastAccessUnix should change
	require.NoError(t, auth_model.UpdateSessionActivity(t.Context(), "sess-activity", "10.0.0.1"))
	got, err := auth_model.GetUserSessionByID(t.Context(), "sess-activity")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", got.LastIP)
	assert.Empty(t, got.PrevIP)

	// Update with new IP — PrevIP should shift
	require.NoError(t, auth_model.UpdateSessionActivity(t.Context(), "sess-activity", "172.16.0.1"))
	got, err = auth_model.GetUserSessionByID(t.Context(), "sess-activity")
	require.NoError(t, err)
	assert.Equal(t, "172.16.0.1", got.LastIP)
	assert.Equal(t, "10.0.0.1", got.PrevIP)

	// Updating a nonexistent session should not error
	require.NoError(t, auth_model.UpdateSessionActivity(t.Context(), "nonexistent", "10.0.0.1"))

	// Updating an already-logged-out session should be a no-op
	require.NoError(t, auth_model.InvalidateUserSession(t.Context(), "sess-activity"))
	beforeUpdate, err := auth_model.GetUserSessionByID(t.Context(), "sess-activity")
	require.NoError(t, err)
	require.NoError(t, auth_model.UpdateSessionActivity(t.Context(), "sess-activity", "192.168.1.1"))
	afterUpdate, err := auth_model.GetUserSessionByID(t.Context(), "sess-activity")
	require.NoError(t, err)
	assert.Equal(t, beforeUpdate.LastIP, afterUpdate.LastIP, "logged-out session IP should not change")
	assert.Equal(t, beforeUpdate.LastAccessUnix, afterUpdate.LastAccessUnix, "logged-out session timestamp should not change")
}

func TestDeleteUserSessionsByUserID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	for _, id := range []string{"sess-del-1", "sess-del-2"} {
		require.NoError(t, auth_model.CreateUserSession(t.Context(), &auth_model.UserSession{
			ID:     id,
			UserID: 4,
		}))
	}

	require.NoError(t, auth_model.DeleteUserSessionsByUserID(t.Context(), 4))

	sessions, err := auth_model.GetUserSessionsByUserID(t.Context(), 4)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestCleanupExpiredUserSessions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	now := timeutil.TimeStampNow()

	// Active session — should survive
	require.NoError(t, auth_model.CreateUserSession(t.Context(), &auth_model.UserSession{
		ID:             "sess-cleanup-active",
		UserID:         1,
		LastAccessUnix: now,
	}))

	// Old logged-out session — should be cleaned up.
	// Use raw engine insert to bypass the "created" auto-fill.
	_, err := db.GetEngine(t.Context()).Insert(&auth_model.UserSession{
		ID:             "sess-cleanup-old",
		UserID:         1,
		LogoutUnix:     timeutil.TimeStamp(int64(now) - 86400*60),
		LastAccessUnix: timeutil.TimeStamp(int64(now) - 86400*60),
		CreatedUnix:    timeutil.TimeStamp(int64(now) - 86400*60),
	})
	require.NoError(t, err)

	retention := 30 * 24 * time.Hour // 30 days
	maxLifetime := 24 * time.Hour    // 1 day
	require.NoError(t, auth_model.CleanupExpiredUserSessions(t.Context(), retention, maxLifetime))

	// Active session should still exist
	_, err = auth_model.GetUserSessionByID(t.Context(), "sess-cleanup-active")
	require.NoError(t, err)

	// Old session should be gone
	_, err = auth_model.GetUserSessionByID(t.Context(), "sess-cleanup-old")
	assert.True(t, auth_model.IsErrUserSessionNotExist(err))
}

func TestCleanupExpiredUserSessionsAbandoned(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	now := timeutil.TimeStampNow()
	retention := 30 * 24 * time.Hour // 30 days
	maxLifetime := 24 * time.Hour    // 1 day

	cutoff := int64(now) - int64(maxLifetime.Seconds()) - int64(retention.Seconds())

	// Abandoned session clearly older than cutoff — should be cleaned up.
	_, err := db.GetEngine(t.Context()).Insert(&auth_model.UserSession{
		ID:             "sess-cleanup-abandoned-old",
		UserID:         1,
		LastAccessUnix: timeutil.TimeStamp(cutoff - 1),
		CreatedUnix:    timeutil.TimeStamp(cutoff - 1),
	})
	require.NoError(t, err)

	// Abandoned session exactly at cutoff — should be preserved (strict < comparison).
	_, err = db.GetEngine(t.Context()).Insert(&auth_model.UserSession{
		ID:             "sess-cleanup-abandoned-boundary",
		UserID:         1,
		LastAccessUnix: timeutil.TimeStamp(cutoff),
		CreatedUnix:    timeutil.TimeStamp(cutoff),
	})
	require.NoError(t, err)

	// Abandoned session newer than cutoff — should be preserved.
	_, err = db.GetEngine(t.Context()).Insert(&auth_model.UserSession{
		ID:             "sess-cleanup-abandoned-new",
		UserID:         1,
		LastAccessUnix: timeutil.TimeStamp(cutoff + 1),
		CreatedUnix:    timeutil.TimeStamp(cutoff + 1),
	})
	require.NoError(t, err)

	require.NoError(t, auth_model.CleanupExpiredUserSessions(t.Context(), retention, maxLifetime))

	// Clearly old abandoned session should be gone.
	_, err = auth_model.GetUserSessionByID(t.Context(), "sess-cleanup-abandoned-old")
	assert.True(t, auth_model.IsErrUserSessionNotExist(err))

	// Boundary and newer abandoned sessions should still exist.
	_, err = auth_model.GetUserSessionByID(t.Context(), "sess-cleanup-abandoned-boundary")
	require.NoError(t, err)

	_, err = auth_model.GetUserSessionByID(t.Context(), "sess-cleanup-abandoned-new")
	require.NoError(t, err)
}
