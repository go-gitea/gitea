// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"
)

// Session represents a session compatible for go-chi session
type Session struct {
	Key    string             `xorm:"pk CHAR(16)"` // has to be Key to match with go-chi/session
	Data   []byte             `xorm:"BLOB"`
	Expiry timeutil.TimeStamp // has to be Expiry to match with go-chi/session
}

// UpdateSession updates the session with provided id
func UpdateSession(key string, data []byte) error {
	_, err := x.ID(key).Update(&Session{
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
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}

	if has, err := sess.Get(&session); err != nil {
		return nil, err
	} else if !has {
		session.Expiry = timeutil.TimeStampNow()
		_, err := sess.Insert(&session)
		if err != nil {
			return nil, err
		}
	}

	return &session, sess.Commit()
}

// ExistSession checks if a session exists
func ExistSession(key string) (bool, error) {
	session := Session{
		Key: key,
	}
	return x.Get(&session)
}

// DestroySession destroys a session
func DestroySession(key string) error {
	_, err := x.Delete(&Session{
		Key: key,
	})
	return err
}

// RegenerateSession regenerates a session from the old id
func RegenerateSession(oldKey, newKey string) (*Session, error) {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}

	if has, err := sess.Get(&Session{
		Key: newKey,
	}); err != nil {
		return nil, err
	} else if has {
		return nil, fmt.Errorf("session Key: %s already exists", newKey)
	}

	if has, err := sess.Get(&Session{
		Key: oldKey,
	}); err != nil {
		return nil, err
	} else if !has {
		_, err := sess.Insert(&Session{
			Key:    oldKey,
			Expiry: timeutil.TimeStampNow(),
		})
		if err != nil {
			return nil, err
		}
	}

	if _, err := sess.Exec("UPDATE "+sess.Engine().TableName(&Session{})+" SET `key` = ? WHERE `key`=?", newKey, oldKey); err != nil {
		return nil, err
	}

	s := Session{
		Key: newKey,
	}
	if _, err := sess.Get(&s); err != nil {
		return nil, err
	}

	return &s, sess.Commit()
}

// CountSessions returns the number of sessions
func CountSessions() (int64, error) {
	return x.Count(&Session{})
}

// CleanupSessions cleans up expired sessions
func CleanupSessions(maxLifetime int64) error {
	_, err := x.Where("expiry <= ?", timeutil.TimeStampNow().Add(-maxLifetime)).Delete(&Session{})
	return err
}
