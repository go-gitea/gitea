// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"

	"xorm.io/builder"
)

type Session struct {
	Key            string             `xorm:"pk CHAR(16)"` // the limit is from legacy go-chi/session
	Data           []byte             `xorm:"BLOB"`        // on MySQL this has a maximum size of 64Kb
	LastAccessTime timeutil.TimeStamp `xorm:"expiry"`      // last access time, the field name is from legacy go-chi/session, we don't want to change it at the moment
}

const DbSessionLastAccessTime = "expiry" // maybe we can make a deeper clean up in the future, just keep this PR focused

func init() {
	db.RegisterModel(new(Session))
}

// UpdateSession stores the data of the session with provided id, creating the session only if create is set
func UpdateSession(ctx context.Context, key string, data []byte, create bool) error {
	session := &Session{Key: key, Data: data, LastAccessTime: timeutil.TimeStampNow()}
	update := func() (int64, error) {
		return db.GetEngine(ctx).ID(key).Cols("data", DbSessionLastAccessTime).Update(session)
	}
	if updated, err := update(); err != nil || updated > 0 || !create {
		return err
	}
	insertErr := db.Insert(ctx, session)
	if insertErr == nil {
		return nil
	}
	// the row exists if a concurrent request inserted it, or if MySQL reported an unchanged row as not updated
	if exist, err := db.Exist[Session](ctx, builder.Eq{"`key`": key}); err != nil || !exist {
		return insertErr
	}
	_, err := update()
	return err
}

func UpdateSessionLastAccessTime(ctx context.Context, key string) error {
	_, err := db.GetEngine(ctx).ID(key).Cols(DbSessionLastAccessTime).Update(&Session{LastAccessTime: timeutil.TimeStampNow()})
	return err
}

func GetSession(ctx context.Context, key string) (*Session, bool, error) {
	return db.Get[Session](ctx, builder.Eq{"`key`": key})
}

// DestroySession destroys a session
func DestroySession(ctx context.Context, key string) error {
	_, err := db.GetEngine(ctx).Delete(&Session{
		Key: key,
	})
	return err
}

// CleanupSessions cleans up expired sessions
func CleanupSessions(ctx context.Context, maxLifetime int64) error {
	_, err := db.GetEngine(ctx).Where(DbSessionLastAccessTime+" <= ?", timeutil.TimeStampNow().Add(-maxLifetime)).Delete(&Session{})
	return err
}
