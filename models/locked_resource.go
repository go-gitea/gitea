// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
)

type LockedResource struct {
	Type	string      `xorm:"pk"`
	Key		int64		`xorm:"pk"`
	Counter	int64		`xorm:"NOT NULL DEFAULT 0"`
}

func GetLockedResource(e Engine, rType string, key int64) (*LockedResource, error) {
	locked := &LockedResource{Type: rType, Key: key}
	has, err := e.Table(locked).ForUpdate().Get(locked)
	if err != nil {
		return nil, fmt.Errorf("get locked resource %s:%d: %v", rType, key, err)
	}
	if !has {
		_, err = e.Insert(locked)
		if err != nil {
			return nil, fmt.Errorf("insert locked resource %s:%d: %v", rType, key, err)
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

func TempLockResource(e Engine, rType string, key int64) error {
	locked := &LockedResource{Type: rType, Key: key}
	// Temporary locked resources must not exist in the table
	_, err := e.Insert(locked)
	if err != nil {
		return fmt.Errorf("insert locked resource %s:%d: %v", rType, key, err)
	}
	// Deleting the resouces prevents it from remain on the table
	// after commit, but keeps the key locked in the database
	// for the duration of the transaction.
	_, err = e.Delete(locked)
	return err
}
