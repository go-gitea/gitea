// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"code.gitea.io/gitea/modules/setting"
)

// ResourceIndex represents a resource index which could be used as issue/release and others
// We can create different tables i.e. issue_index, release_index, etc.
type ResourceIndex struct {
	GroupID  int64 `xorm:"pk"`
	MaxIndex int64 `xorm:"index"`
}

var (
	// ErrResouceOutdated represents an error when request resource outdated
	ErrResouceOutdated = errors.New("resource outdated")
	// ErrGetResourceIndexFailed represents an error when resource index retries 3 times
	ErrGetResourceIndexFailed = errors.New("get resource index failed")
)

// SyncMaxResourceIndex sync the max index with the resource
func SyncMaxResourceIndex(ctx context.Context, tableName string, groupID, maxIndex int64) (err error) {
	e := GetEngine(ctx)

	// try to update the max_index and acquire the write-lock for the record
	res, err := e.Exec(fmt.Sprintf("UPDATE %s SET max_index=? WHERE group_id=? AND max_index<?", tableName), maxIndex, groupID, maxIndex)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// if nothing is updated, the record might not exist or might be larger, it's safe to try to insert it again and then check whether the record exists
		_, errIns := e.Exec(fmt.Sprintf("INSERT INTO %s (group_id, max_index) VALUES (?, ?)", tableName), groupID, maxIndex)
		var savedIdx int64
		has, err := e.SQL(fmt.Sprintf("SELECT max_index FROM %s WHERE group_id=?", tableName), groupID).Get(&savedIdx)
		if err != nil {
			return err
		}
		// if the record still doesn't exist, there must be some errors (insert error)
		if !has {
			if errIns == nil {
				return errors.New("impossible error when SyncMaxResourceIndex, insert succeeded but no record is saved")
			}
			return errIns
		}
	}
	return nil
}

func postgresGetNextResourceIndex(ctx context.Context, tableName string, groupID int64) (int64, error) {
	res, err := GetEngine(ctx).Query(fmt.Sprintf("INSERT INTO %s (group_id, max_index) "+
		"VALUES (?,1) ON CONFLICT (group_id) DO UPDATE SET max_index = %s.max_index+1 RETURNING max_index",
		tableName, tableName), groupID)
	if err != nil {
		return 0, err
	}
	if len(res) == 0 {
		return 0, ErrGetResourceIndexFailed
	}
	return strconv.ParseInt(string(res[0]["max_index"]), 10, 64)
}

func mysqlGetNextResourceIndex(ctx context.Context, tableName string, groupID int64) (int64, error) {
	if _, err := GetEngine(ctx).Exec(fmt.Sprintf("INSERT INTO %s (group_id, max_index) "+
		"VALUES (?,1) ON DUPLICATE KEY UPDATE max_index = max_index+1",
		tableName), groupID); err != nil {
		return 0, err
	}

	var idx int64
	_, err := GetEngine(ctx).SQL(fmt.Sprintf("SELECT max_index FROM %s WHERE group_id = ?", tableName), groupID).Get(&idx)
	if err != nil {
		return 0, err
	}
	if idx == 0 {
		return 0, errors.New("cannot get the correct index")
	}
	return idx, nil
}

func mssqlGetNextResourceIndex(ctx context.Context, tableName string, groupID int64) (int64, error) {
	if _, err := GetEngine(ctx).Exec(fmt.Sprintf(`
MERGE INTO %s WITH (HOLDLOCK) AS target
USING (SELECT %d AS group_id) AS source
(group_id)
ON target.group_id = source.group_id
WHEN MATCHED
	THEN UPDATE
			SET max_index = max_index + 1
WHEN NOT MATCHED
	THEN INSERT (group_id, max_index)
			VALUES (%d, 1);
`, tableName, groupID, groupID)); err != nil {
		return 0, err
	}

	var idx int64
	_, err := GetEngine(ctx).SQL(fmt.Sprintf("SELECT max_index FROM %s WHERE group_id = ?", tableName), groupID).Get(&idx)
	if err != nil {
		return 0, err
	}
	if idx == 0 {
		return 0, errors.New("cannot get the correct index")
	}
	return idx, nil
}

// GetNextResourceIndex generates a resource index, it must run in the same transaction where the resource is created
func GetNextResourceIndex(ctx context.Context, tableName string, groupID int64) (int64, error) {
	switch {
	case setting.Database.Type.IsPostgreSQL():
		return postgresGetNextResourceIndex(ctx, tableName, groupID)
	case setting.Database.Type.IsMySQL():
		return mysqlGetNextResourceIndex(ctx, tableName, groupID)
	case setting.Database.Type.IsMSSQL():
		return mssqlGetNextResourceIndex(ctx, tableName, groupID)
	}

	e := GetEngine(ctx)

	// try to update the max_index to next value, and acquire the write-lock for the record
	res, err := e.Exec(fmt.Sprintf("UPDATE %s SET max_index=max_index+1 WHERE group_id=?", tableName), groupID)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if affected == 0 {
		// this slow path is only for the first time of creating a resource index
		_, errIns := e.Exec(fmt.Sprintf("INSERT INTO %s (group_id, max_index) VALUES (?, 0)", tableName), groupID)
		res, err = e.Exec(fmt.Sprintf("UPDATE %s SET max_index=max_index+1 WHERE group_id=?", tableName), groupID)
		if err != nil {
			return 0, err
		}
		affected, err = res.RowsAffected()
		if err != nil {
			return 0, err
		}
		// if the update still can not update any records, the record must not exist and there must be some errors (insert error)
		if affected == 0 {
			if errIns == nil {
				return 0, errors.New("impossible error when GetNextResourceIndex, insert and update both succeeded but no record is updated")
			}
			return 0, errIns
		}
	}

	// now, the new index is in database (protected by the transaction and write-lock)
	var newIdx int64
	has, err := e.SQL(fmt.Sprintf("SELECT max_index FROM %s WHERE group_id=?", tableName), groupID).Get(&newIdx)
	if err != nil {
		return 0, err
	}
	if !has {
		return 0, errors.New("impossible error when GetNextResourceIndex, upsert succeeded but no record can be selected")
	}
	return newIdx, nil
}

// DeleteResourceIndex delete resource index
func DeleteResourceIndex(ctx context.Context, tableName string, groupID int64) error {
	_, err := Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE group_id=?", tableName), groupID)
	return err
}
