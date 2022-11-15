// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
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
func UpdateSession(key string, data []byte) error {
	_, err := db.GetEngine(db.DefaultContext).ID(key).Update(&Session{
		Data:   data,
		Expiry: timeutil.TimeStampNow(),
	})
	return err
}

// ReadSession reads the data for the provided session
func ReadSession(key string) (*Session, error) {
	session := Session{
		Key: key,
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	if has, err := db.GetByBean(ctx, &session); err != nil {
		return nil, err
	} else if !has {
		session.Expiry = timeutil.TimeStampNow()
		if err := db.Insert(ctx, &session); err != nil {
			return nil, err
		}
	}

	return &session, committer.Commit()
}

// ExistSession checks if a session exists
func ExistSession(key string) (bool, error) {
	session := Session{
		Key: key,
	}
	return db.GetEngine(db.DefaultContext).Get(&session)
}

// DestroySession destroys a session
func DestroySession(key string) error {
	_, err := db.GetEngine(db.DefaultContext).Delete(&Session{
		Key: key,
	})
	return err
}

// RegenerateSession regenerates a session from the old id
func RegenerateSession(oldKey, newKey string) (*Session, error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	if has, err := db.GetByBean(ctx, &Session{
		Key: newKey,
	}); err != nil {
		return nil, err
	} else if has {
		return nil, fmt.Errorf("session Key: %s already exists", newKey)
	}

	if has, err := db.GetByBean(ctx, &Session{
		Key: oldKey,
	}); err != nil {
		return nil, err
	} else if !has {
		if err := db.Insert(ctx, &Session{
			Key:    oldKey,
			Expiry: timeutil.TimeStampNow(),
		}); err != nil {
			return nil, err
		}
	}

	if _, err := db.Exec(ctx, "UPDATE "+db.TableName(&Session{})+" SET `key` = ? WHERE `key`=?", newKey, oldKey); err != nil {
		return nil, err
	}

	s := Session{
		Key: newKey,
	}
	if _, err := db.GetByBean(ctx, &s); err != nil {
		return nil, err
	}

	return &s, committer.Commit()
}

// CountSessions returns the number of sessions
func CountSessions() (int64, error) {
	return db.GetEngine(db.DefaultContext).Count(&Session{})
}

// CleanupSessions cleans up expired sessions
func CleanupSessions(maxLifetime int64) error {
	_, err := db.GetEngine(db.DefaultContext).Where("expiry <= ?", timeutil.TimeStampNow().Add(-maxLifetime)).Delete(&Session{})
	return err
}
