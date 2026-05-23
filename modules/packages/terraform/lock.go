// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform

import (
	"context"
	"errors"
	"io"
	"time"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

const LockFile = "terraform.lock"

// LockInfo is the metadata for a terraform lock.
type LockInfo struct {
	ID        string    `json:"ID"`
	Operation string    `json:"Operation"`
	Info      string    `json:"Info"`
	Who       string    `json:"Who"`
	Version   string    `json:"Version"`
	Created   time.Time `json:"Created"`
	Path      string    `json:"Path"`
}

func (l *LockInfo) IsLocked() bool {
	return l.ID != ""
}

func ParseLockInfo(r io.Reader) (*LockInfo, error) {
	var lock LockInfo
	err := json.NewDecoder(r).Decode(&lock)
	if err != nil {
		return nil, err
	}
	// ID is required. Rest is less important.
	if lock.ID == "" {
		return nil, util.NewInvalidArgumentErrorf("terraform lock is missing an ID")
	}
	return &lock, nil
}

// GetLock returns the terraform lock for the given package.
// Lock is empty if no lock exists.
func GetLock(ctx context.Context, packageID int64) (LockInfo, error) {
	var lock LockInfo
	locks, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypePackage, packageID, LockFile)
	if err != nil {
		return lock, err
	}
	if len(locks) == 0 || locks[0].Value == "" {
		return lock, nil
	}

	err = json.Unmarshal([]byte(locks[0].Value), &lock)
	return lock, err
}

// SetLock sets the terraform lock for the given package.
func SetLock(ctx context.Context, packageID int64, lock *LockInfo) error {
	jsonBytes, err := json.Marshal(lock)
	if err != nil {
		return err
	}

	return updateLock(ctx, packageID, string(jsonBytes), builder.Eq{"value": ""})
}

// RemoveLock removes the terraform lock for the given package.
func RemoveLock(ctx context.Context, packageID int64) error {
	return updateLock(ctx, packageID, "", builder.Neq{"value": ""})
}

func updateLock(ctx context.Context, refID int64, value string, cond builder.Cond) error {
	pp := packages_model.PackageProperty{RefType: packages_model.PropertyTypePackage, RefID: refID, Name: LockFile}
	ok, err := db.GetEngine(ctx).Get(&pp)
	if err != nil {
		return err
	}
	if ok {
		n, err := db.GetEngine(ctx).Where("ref_type=? AND ref_id=? AND name=?", packages_model.PropertyTypePackage, refID, LockFile).And(cond).Cols("value").Update(&packages_model.PackageProperty{Value: value})
		if err != nil {
			return err
		}
		if n == 0 {
			return errors.New("failed to update lock state")
		}

		return nil
	}
	_, err = packages_model.InsertProperty(ctx, packages_model.PropertyTypePackage, refID, LockFile, value)
	return err
}
