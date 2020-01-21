// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/setting"
)

type LockedResource struct {
	LockType	string      `xorm:"pk VARCHAR(30)"`
	LockKey		int64		`xorm:"pk"`
	Counter		int64		`xorm:"NOT NULL DEFAULT 0"`
}

func GetLockedResource(e Engine, lockType string, lockKey int64) (*LockedResource, error) {
	locked := &LockedResource{LockType: lockType, LockKey: lockKey}

	if err := upsertLockedResource(e, locked); err != nil {
		return nil, fmt.Errorf("upsertLockedResource: %v", err)
	}

	// Read back the record we've created or locked to get the current Counter value
	if has, err := e.Table(locked).Get(locked); err != nil {
		return nil, fmt.Errorf("get locked resource %s:%d: %v", lockType, lockKey, err)
	} else if !has {
		return nil, fmt.Errorf("unexpected upsert fail  %s:%d", lockType, lockKey)
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

func TempLockResource(e Engine, lockType string, lockKey int64) error {
	locked := &LockedResource{LockType: lockType, LockKey: lockKey}
	// Temporary locked resources must not exist in the table.
	// This allows us to use a simple INSERT to lock the key.
	_, err := e.Insert(locked)
	if err == nil {
		_, err = e.Delete(locked)
	}
	return err
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

func TempLockResourceCtx(ctx DBContext, lockType string, lockKey int64) error {
	return TempLockResource(ctx.e, lockType, lockKey)
}

func upsertLockedResource(e Engine, resource *LockedResource) (err error) {
	// An atomic UPSERT operation (INSERT/UPDATE) is the only operation
	// that ensures that the key is actually locked. 
	switch {
	case setting.Database.UseSQLite3 || setting.Database.UsePostgreSQL:
		_, err = e.Exec("INSERT INTO locked_resource (lock_type, lock_key) "+
			"VALUES (?,?) ON CONFLICT(lock_type, lock_key) DO UPDATE SET lock_key = ?",
			resource.LockType, resource.LockKey, resource.LockKey);
	case setting.Database.UseMySQL:
		_, err = e.Exec("INSERT INTO locked_resource (lock_type, lock_key) "+
			"VALUES (?,?) ON DUPLICATE KEY UPDATE lock_key = lock_key",
			resource.LockType, resource.LockKey);
	case setting.Database.UseMSSQL:
		// https://weblogs.sqlteam.com/dang/2009/01/31/upsert-race-condition-with-merge/
		_, err = e.Exec("MERGE locked_resource WITH (HOLDLOCK) as target "+
			"USING (SELECT ? AS lock_type, ? AS lock_key) AS src "+
			"ON src.lock_type = target.lock_type AND src.lock_key = target.lock_key "+
			"WHEN MATCHED THEN UPDATE SET target.lock_key = target.lock_key "+
			"WHEN NOT MATCHED THEN INSERT (lock_type, lock_key) "+
			"VALUES (src.lock_type, src.lock_key);",
			resource.LockType, resource.LockKey);
	default:
		return fmt.Errorf("database type not supported")
	}
	return
}