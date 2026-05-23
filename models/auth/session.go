// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// Session represents a session compatible for go-chi session
type Session struct {
	Key    string             `xorm:"pk CHAR(16)"` // has to be Key to match with go-chi/session
	Data   []byte             `xorm:"BLOB"`        // on MySQL this has a maximum size of 64Kb - this may need to be increased
	Expiry timeutil.TimeStamp // has to be Expiry to match with go-chi/session
}

func init() {
	db.RegisterModel(new(Session))
}

// UpdateSession updates the session with provided id
func UpdateSession(ctx context.Context, key string, data []byte) error {
	_, err := db.GetEngine(ctx).ID(key).Update(&Session{
		Data:   data,
		Expiry: timeutil.TimeStampNow(),
	})
	return err
}

// ReadSession reads the data for the provided session
func ReadSession(ctx context.Context, key string) (*Session, error) {
	return db.WithTx2(ctx, func(ctx context.Context) (*Session, error) {
		session, exist, err := db.Get[Session](ctx, builder.Eq{"`key`": key})
		if err != nil {
			return nil, err
		} else if !exist {
			session = &Session{
				Key:    key,
				Expiry: timeutil.TimeStampNow(),
			}
			if err := db.Insert(ctx, session); err != nil {
				return nil, err
			}
		}

		return session, nil
	})
}

// ExistSession checks if a session exists
func ExistSession(ctx context.Context, key string) (bool, error) {
	return db.Exist[Session](ctx, builder.Eq{"`key`": key})
}

// DestroySession destroys a session
func DestroySession(ctx context.Context, key string) error {
	_, err := db.GetEngine(ctx).Delete(&Session{
		Key: key,
	})
	return err
}

// RegenerateSession regenerates a session from the old id
func RegenerateSession(ctx context.Context, oldKey, newKey string) (*Session, error) {
	return db.WithTx2(ctx, func(ctx context.Context) (*Session, error) {
		if has, err := db.Exist[Session](ctx, builder.Eq{"`key`": newKey}); err != nil {
			return nil, err
		} else if has {
			return nil, fmt.Errorf("session Key: %s already exists", newKey)
		}

		if has, err := db.Exist[Session](ctx, builder.Eq{"`key`": oldKey}); err != nil {
			return nil, err
		} else if !has {
			if err := db.Insert(ctx, &Session{
				Key:    oldKey,
				Expiry: timeutil.TimeStampNow(),
			}); err != nil {
				return nil, err
			}
		}

		if _, err := db.Exec(ctx, "UPDATE `session` SET `key` = ? WHERE `key`=?", newKey, oldKey); err != nil {
			return nil, err
		}

		s, _, err := db.Get[Session](ctx, builder.Eq{"`key`": newKey})
		if err != nil {
			// is not exist, it should be impossible
			return nil, err
		}

		return s, nil
	})
}

// CountSessions returns the number of sessions
func CountSessions(ctx context.Context) (int64, error) {
	return db.GetEngine(ctx).Count(&Session{})
}

// CleanupSessions cleans up expired sessions
func CleanupSessions(ctx context.Context, maxLifetime int64) error {
	_, err := db.GetEngine(ctx).Where("expiry <= ?", timeutil.TimeStampNow().Add(-maxLifetime)).Delete(&Session{})
	return err
}
