// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"
)

// Session represents a session compatible for go-macaron session
type Session struct {
	ID     string             `xorm:"pk CHAR(16)"`
	Data   []byte             `xorm:"BLOB"`
	Expiry timeutil.TimeStamp // has to be Expiry to match with go-macaron/session
}

// UpdateSession updates the session with provided id
func UpdateSession(id string, data []byte) error {
	_, err := x.Update(&Session{
		ID:     id,
		Data:   data,
		Expiry: timeutil.TimeStampNow(),
	})
	return err
}

// ReadSession reads the data for the provided session
func ReadSession(id string) (*Session, error) {
	session := Session{
		ID: id,
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
func ExistSession(id string) (bool, error) {
	session := Session{
		ID: id,
	}
	return x.Get(&session)
}

// DestroySession destroys a session
func DestroySession(id string) error {
	_, err := x.Delete(&Session{
		ID: id,
	})
	return err
}

// RegenerateSession regenerates a session from the old id
func RegenerateSession(oldID, newID string) (*Session, error) {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}

	if has, err := sess.Get(&Session{
		ID: newID,
	}); err != nil {
		return nil, err
	} else if has {
		return nil, fmt.Errorf("session ID: %s already exists", newID)
	}

	if has, err := sess.Get(&Session{
		ID: oldID,
	}); err != nil {
		return nil, err
	} else if !has {
		_, err := sess.Insert(&Session{
			ID:     oldID,
			Expiry: timeutil.TimeStampNow(),
		})
		if err != nil {
			return nil, err
		}
	}

	if _, err := sess.Exec("UPDATE "+x.TableName(&Session{})+" SET `id` = ? WHERE `id`=?", newID, oldID); err != nil {
		return nil, err
	}

	s := Session{
		ID: newID,
	}
	if _, err := sess.Get(s); err != nil {
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
	_, err := x.Where("created_unix <= ?", timeutil.TimeStampNow().Add(-maxLifetime)).Delete(&Session{})
	return err
}
