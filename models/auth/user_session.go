// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrUserSessionNotExist is returned when a user session does not exist
type ErrUserSessionNotExist struct {
	ID string
}

func (err ErrUserSessionNotExist) Error() string {
	return fmt.Sprintf("user session does not exist [id: %s]", err.ID)
}

func (err ErrUserSessionNotExist) Unwrap() error {
	return util.ErrNotExist
}

// IsErrUserSessionNotExist checks if an error is ErrUserSessionNotExist
func IsErrUserSessionNotExist(err error) bool {
	_, ok := err.(ErrUserSessionNotExist)
	return ok
}

// UserSession represents a tracked user session with metadata
type UserSession struct {
	ID             string             `xorm:"pk VARCHAR(64)"`
	UserID         int64              `xorm:"INDEX NOT NULL"`
	LoginIP        string             `xorm:"VARCHAR(45)"`
	LastIP         string             `xorm:"VARCHAR(45)"`
	PrevIP         string             `xorm:"VARCHAR(45)"`
	UserAgent      string             `xorm:"TEXT"`
	LoginMethod    string             `xorm:"VARCHAR(64)"`
	AuthTokenID    string             `xorm:"VARCHAR(64)"`
	CreatedUnix    timeutil.TimeStamp `xorm:"INDEX NOT NULL created"`
	LastAccessUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL"`
	LogoutUnix     timeutil.TimeStamp `xorm:"INDEX NOT NULL DEFAULT 0"`
}

func init() {
	db.RegisterModel(new(UserSession))
}

// CreateUserSession inserts a new user session record
func CreateUserSession(ctx context.Context, session *UserSession) error {
	return db.Insert(ctx, session)
}

// GetUserSessionByID returns a single session by its ID
func GetUserSessionByID(ctx context.Context, id string) (*UserSession, error) {
	sess, has, err := db.Get[UserSession](ctx, builder.Eq{"id": id})
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrUserSessionNotExist{ID: id}
	}
	return sess, nil
}

// GetUserSessionsByUserID returns all sessions for a user, ordered by creation time descending
func GetUserSessionsByUserID(ctx context.Context, userID int64) ([]*UserSession, error) {
	sessions := make([]*UserSession, 0, 8)
	return sessions, db.GetEngine(ctx).Where("user_id = ?", userID).
		Desc("created_unix").Find(&sessions)
}

// CountUserSessionsByUserID returns (total, active) session counts for a user
// without materializing the rows.
func CountUserSessionsByUserID(ctx context.Context, userID int64) (total, active int64, err error) {
	e := db.GetEngine(ctx)
	total, err = e.Where("user_id = ?", userID).Count(new(UserSession))
	if err != nil {
		return 0, 0, err
	}
	active, err = e.Where("user_id = ? AND logout_unix = 0", userID).Count(new(UserSession))
	if err != nil {
		return 0, 0, err
	}
	return total, active, nil
}

// InvalidateUserSession marks a session as logged out
func InvalidateUserSession(ctx context.Context, sessionID string) error {
	_, err := db.GetEngine(ctx).Where("id = ? AND logout_unix = 0", sessionID).
		Cols("logout_unix").
		Update(&UserSession{LogoutUnix: timeutil.TimeStampNow()})
	return err
}

// InvalidateAllUserSessions marks all active sessions for a user as logged out,
// optionally excluding a specific session
func InvalidateAllUserSessions(ctx context.Context, userID int64, exceptSessionID string) error {
	sess := db.GetEngine(ctx).Where("user_id = ? AND logout_unix = 0", userID)
	if exceptSessionID != "" {
		sess = sess.And("id != ?", exceptSessionID)
	}
	_, err := sess.Cols("logout_unix").Update(&UserSession{LogoutUnix: timeutil.TimeStampNow()})
	return err
}

// UpdateSessionActivity updates the last access time and IP shift logic
// using a single UPDATE statement with no prior SELECT.
// Only updates sessions that are still active (not yet logged out).
func UpdateSessionActivity(ctx context.Context, sessionID, currentIP string) error {
	now := int64(timeutil.TimeStampNow())
	if currentIP == "" {
		_, err := db.GetEngine(ctx).Exec(
			"UPDATE user_session SET last_access_unix = ? WHERE id = ? AND logout_unix = 0",
			now, sessionID,
		)
		return err
	}
	_, err := db.GetEngine(ctx).Exec(
		"UPDATE user_session SET last_access_unix = ?,"+
			" prev_ip = CASE WHEN last_ip != ? AND last_ip != '' THEN last_ip ELSE prev_ip END,"+
			" last_ip = ? WHERE id = ? AND logout_unix = 0",
		now, currentIP, currentIP, sessionID,
	)
	return err
}

// CleanupExpiredUserSessions removes old session records based on retention policy.
// It deletes:
// - Sessions that were logged out more than retention ago
// - Abandoned sessions (never logged out) whose last activity is older than maxLifetime + retention
func CleanupExpiredUserSessions(ctx context.Context, retention, maxLifetime time.Duration) error {
	now := int64(timeutil.TimeStampNow())
	logoutCutoff := now - int64(retention.Seconds())
	abandonedCutoff := now - int64(maxLifetime.Seconds()) - int64(retention.Seconds())

	_, err := db.GetEngine(ctx).Where(
		builder.Or(
			builder.And(builder.Gt{"logout_unix": 0}, builder.Lt{"logout_unix": logoutCutoff}),
			builder.And(builder.Eq{"logout_unix": 0}, builder.Lt{"last_access_unix": abandonedCutoff}),
		),
	).Delete(&UserSession{})
	return err
}

// DeleteUserSessionsByUserID removes all session records for a user (used on user deletion)
func DeleteUserSessionsByUserID(ctx context.Context, userID int64) error {
	_, err := db.GetEngine(ctx).Where("user_id = ?", userID).Delete(&UserSession{})
	return err
}
