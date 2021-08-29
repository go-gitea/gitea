// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"errors"
	"fmt"

	"code.gitea.io/gitea/modules/setting"
)

// ResourceIndex represents a resource index which could be used as issue/release and others
// We can create different tables i.e. issue_index, release_index and etc.
type ResourceIndex struct {
	GroupID  int64 `xorm:"pk"`
	MaxIndex int64 `xorm:"index"`
}

// IssueIndex represents the issue index table
type IssueIndex ResourceIndex

// upsertResourceIndex the function will not return until it acquires the lock or receives an error.
func upsertResourceIndex(e Engine, tableName string, groupID int64) (err error) {
	// An atomic UPSERT operation (INSERT/UPDATE) is the only operation
	// that ensures that the key is actually locked.
	switch {
	case setting.Database.UseSQLite3 || setting.Database.UsePostgreSQL:
		_, err = e.Exec(fmt.Sprintf("INSERT INTO %s (group_id, max_index) "+
			"VALUES (?,1) ON CONFLICT (group_id) DO UPDATE SET max_index = %s.max_index+1",
			tableName, tableName), groupID)
	case setting.Database.UseMySQL:
		_, err = e.Exec(fmt.Sprintf("INSERT INTO %s (group_id, max_index) "+
			"VALUES (?,1) ON DUPLICATE KEY UPDATE max_index = max_index+1", tableName),
			groupID)
	case setting.Database.UseMSSQL:
		// https://weblogs.sqlteam.com/dang/2009/01/31/upsert-race-condition-with-merge/
		_, err = e.Exec(fmt.Sprintf("MERGE %s WITH (HOLDLOCK) as target "+
			"USING (SELECT ? AS group_id) AS src "+
			"ON src.group_id = target.group_id "+
			"WHEN MATCHED THEN UPDATE SET target.max_index = target.max_index+1 "+
			"WHEN NOT MATCHED THEN INSERT (group_id, max_index) "+
			"VALUES (src.group_id, 1);", tableName),
			groupID)
	default:
		return fmt.Errorf("database type not supported")
	}
	return
}

var (
	// ErrResouceOutdated represents an error when request resource outdated
	ErrResouceOutdated = errors.New("resource outdated")
	// ErrGetResourceIndexFailed represents an error when resource index retries 3 times
	ErrGetResourceIndexFailed = errors.New("get resource index failed")
)

const (
	maxDupIndexAttempts = 3
)

// GetNextResourceIndex retried 3 times to generate a resource index
func GetNextResourceIndex(tableName string, groupID int64) (int64, error) {
	for i := 0; i < maxDupIndexAttempts; i++ {
		idx, err := getNextResourceIndex(tableName, groupID)
		if err == ErrResouceOutdated {
			continue
		}
		if err != nil {
			return 0, err
		}
		return idx, nil
	}
	return 0, ErrGetResourceIndexFailed
}

// deleteResouceIndex delete resource index
func deleteResouceIndex(e Engine, tableName string, groupID int64) error {
	_, err := e.Exec(fmt.Sprintf("DELETE FROM %s WHERE group_id=?", tableName), groupID)
	return err
}

// getNextResourceIndex return the next index
func getNextResourceIndex(tableName string, groupID int64) (int64, error) {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return 0, err
	}
	var preIdx int64
	_, err := sess.SQL(fmt.Sprintf("SELECT max_index FROM %s WHERE group_id = ?", tableName), groupID).Get(&preIdx)
	if err != nil {
		return 0, err
	}

	if err := upsertResourceIndex(sess, tableName, groupID); err != nil {
		return 0, err
	}

	var curIdx int64
	has, err := sess.SQL(fmt.Sprintf("SELECT max_index FROM %s WHERE group_id = ? AND max_index=?", tableName), groupID, preIdx+1).Get(&curIdx)
	if err != nil {
		return 0, err
	}
	if !has {
		return 0, ErrResouceOutdated
	}
	if err := sess.Commit(); err != nil {
		return 0, err
	}
	return curIdx, nil
}
