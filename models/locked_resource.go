// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/setting"
)

// LockedResource represents the locking key for a pessimistic
// lock that can hold a counter
type LockedResource struct {
	LockType string `xorm:"pk VARCHAR(30)"`
	LockKey  int64  `xorm:"pk"`
	Counter  int64  `xorm:"NOT NULL DEFAULT 0"`

	engine Engine `xorm:"-"`
}

// GetLockedResource gets or creates a pessimistic lock on the given type and key
func GetLockedResource(e Engine, lockType string, lockKey int64) (*LockedResource, error) {
	resource := &LockedResource{LockType: lockType, LockKey: lockKey}

	if err := upsertLockedResource(e, resource); err != nil {
		return nil, fmt.Errorf("upsertLockedResource: %v", err)
	}

	// Read back the record we've created or locked to get the current Counter value
	if has, err := e.Table(resource).NoCache().NoAutoCondition().AllCols().
		Where("lock_type = ? AND lock_key = ?", lockType, lockKey).Get(resource); err != nil {
		return nil, fmt.Errorf("get locked resource %s:%d: %v", lockType, lockKey, err)
	} else if !has {
		return nil, fmt.Errorf("unexpected upsert fail  %s:%d", lockType, lockKey)
	}

	// Once active, the locked resource is tied to a specific session
	resource.engine = e

	return resource, nil
}

// UpdateValue updates the value of the counter of a locked resource
func (r *LockedResource) UpdateValue() error {
	// Bypass ORM to support lock_type == "" and lock_key == 0
	_, err := r.engine.Exec("UPDATE locked_resource SET counter = ? WHERE lock_type = ? AND lock_key = ?",
		r.Counter, r.LockType, r.LockKey)
	return err
}

// Delete deletes the locked resource from the database,
// but the key remains locked until the end of the transaction
func (r *LockedResource) Delete() error {
	// Bypass ORM to support lock_type == "" and lock_key == 0
	_, err := r.engine.Exec("DELETE FROM locked_resource WHERE lock_type = ? AND lock_key = ?", r.LockType, r.LockKey)
	return err
}

// DeleteLockedResourceKey deletes a locked resource by key
func DeleteLockedResourceKey(e Engine, lockType string, lockKey int64) error {
	// Bypass ORM to support lock_type == "" and lock_key == 0
	_, err := e.Exec("DELETE FROM locked_resource WHERE lock_type = ? AND lock_key = ?", lockType, lockKey)
	return err
}

// TemporarilyLockResourceKey locks the given key but does not leave a permanent record
func TemporarilyLockResourceKey(e Engine, lockType string, lockKey int64) error {
	// Temporary locked resources should not exist in the table.
	// This allows us to use a simple INSERT to lock the key.
	_, err := e.Exec("INSERT INTO locked_resource (lock_type, lock_key) VALUES (?, ?)", lockType, lockKey)
	if err == nil {
		_, err = e.Exec("DELETE FROM locked_resource WHERE lock_type = ? AND lock_key = ?", lockType, lockKey)
	}
	return err
}

// GetLockedResourceCtx gets or creates a pessimistic lock on the given type and key
func GetLockedResourceCtx(ctx DBContext, lockType string, lockKey int64) (*LockedResource, error) {
	return GetLockedResource(ctx.e, lockType, lockKey)
}

// DeleteLockedResourceKeyCtx deletes a locked resource by key
func DeleteLockedResourceKeyCtx(ctx DBContext, lockType string, lockKey int64) error {
	return DeleteLockedResourceKey(ctx.e, lockType, lockKey)
}

// TemporarilyLockResourceKeyCtx locks the given key but does not leave a permanent record
func TemporarilyLockResourceKeyCtx(ctx DBContext, lockType string, lockKey int64) error {
	return TemporarilyLockResourceKey(ctx.e, lockType, lockKey)
}

// upsertLockedResource will create or lock the given key in the database.
// the function will not return until it acquires the lock or receives an error.
func upsertLockedResource(e Engine, resource *LockedResource) (err error) {
	// An atomic UPSERT operation (INSERT/UPDATE) is the only operation
	// that ensures that the key is actually locked.
	switch {
	case setting.Database.UseSQLite3 || setting.Database.UsePostgreSQL:
		_, err = e.Exec("INSERT INTO locked_resource (lock_type, lock_key) "+
			"VALUES (?,?) ON CONFLICT(lock_type, lock_key) DO UPDATE SET lock_key = ?",
			resource.LockType, resource.LockKey, resource.LockKey)
	case setting.Database.UseMySQL:
		_, err = e.Exec("INSERT INTO locked_resource (lock_type, lock_key) "+
			"VALUES (?,?) ON DUPLICATE KEY UPDATE lock_key = lock_key",
			resource.LockType, resource.LockKey)
	case setting.Database.UseMSSQL:
		// https://weblogs.sqlteam.com/dang/2009/01/31/upsert-race-condition-with-merge/
		_, err = e.Exec("MERGE locked_resource WITH (HOLDLOCK) as target "+
			"USING (SELECT ? AS lock_type, ? AS lock_key) AS src "+
			"ON src.lock_type = target.lock_type AND src.lock_key = target.lock_key "+
			"WHEN MATCHED THEN UPDATE SET target.lock_key = target.lock_key "+
			"WHEN NOT MATCHED THEN INSERT (lock_type, lock_key) "+
			"VALUES (src.lock_type, src.lock_key);",
			resource.LockType, resource.LockKey)
	default:
		return fmt.Errorf("database type not supported")
	}
	return
}
