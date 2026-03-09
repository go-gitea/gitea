// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package terraform

import (
	"context"
	"time"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/json"
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

	return packages_model.InsertOrUpdateProperty(ctx, packages_model.PropertyTypePackage, packageID, LockFile, string(jsonBytes))
}

// RemoveLock removes the terraform lock for the given package.
func RemoveLock(ctx context.Context, packageID int64) error {
	return packages_model.InsertOrUpdateProperty(ctx, packages_model.PropertyTypePackage, packageID, LockFile, "")
}
