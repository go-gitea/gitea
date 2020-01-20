// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
)

type LockedResource struct {
	LockType	string      `xorm:"pk"`
	LockKey		int64		`xorm:"pk"`
	Counter		int64		`xorm:"NOT NULL DEFAULT 0"`
}

func GetLockedResource(e Engine, lockType string, lockKey int64) (*LockedResource, error) {
	locked := &LockedResource{LockType: lockType, LockKey: lockKey}
	// SQLite3 has no ForUpdate() clause and an UPSERT strategy has many
	// problems and fallbacks; we perform a bogus update on the table
	// which will lock the key in a safe way.
	// Make sure to leave `counter` out of the update.
	count, err := e.Table(locked).Cols("lock_type", "lock_key").Update(locked)
	if err != nil {
		return nil, fmt.Errorf("get locked resource %s:%d: %v", lockType, lockKey, err)
	}
	if count == 0 {
		// No record was found; since the key is now locked,
		// it's safe to insert a record.
		_, err = e.Insert(locked)
		if err != nil {
			return nil, fmt.Errorf("get locked resource %s:%d: %v", lockType, lockKey, err)
		}
	} else {
		// Read back the record we've locked
		has, err := e.Table(locked).Get(locked)
		if err != nil {
			return nil, fmt.Errorf("get locked resource %s:%d: %v", lockType, lockKey, err)
		}
		if !has {
			return nil, fmt.Errorf("get locked resource %s:%d: record not found", lockType, lockKey)
		}
	}
	return locked, nil
}

func UpdateLockedResource(e Engine, resource *LockedResource) error {
	_, err := e.Table(resource).Cols("counter").Update(resource)
	return err
}

func DeleteLockedResource(e Engine, resource *LockedResource) error {
	_, err := e.Delete(resource)
	return err
}

func TempLockResource(e Engine, lockType string, lockKey int64) (func() error, error) {
	locked := &LockedResource{LockType: lockType, LockKey: lockKey}
	// Temporary locked resources must not exist in the table
	_, err := e.Insert(locked)
	if err != nil {
		return func() error {
			return nil
		},
		fmt.Errorf("insert locked resource %s:%d: %v", lockType, lockKey, err)
	}
	return func() error {
		_, err := e.Delete(locked)
		return err
	}, nil
}

func GetLockedResourceCtx(ctx DBContext, lockType string, lockKey int64) (*LockedResource, error) {
	return GetLockedResource(ctx.e, lockType, lockKey)
}

func UpdateLockedResourceCtx(ctx DBContext, resource *LockedResource) error {
	return UpdateLockedResource(ctx.e, resource)
}

func DeleteLockedResourceCtx(ctx DBContext, resource *LockedResource) error {
	return DeleteLockedResource(ctx.e, resource)
}

func TempLockResourceCtx(ctx DBContext, lockType string, lockKey int64) (func() error, error) {
	return TempLockResource(ctx.e, lockType, lockKey)
}

