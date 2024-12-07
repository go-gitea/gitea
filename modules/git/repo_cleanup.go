// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func ForciblyUnlockRepository(ctx context.Context, repoPath string) error {
	return cleanLocksIfNeeded(repoPath, time.Now())
}

func ForciblyUnlockRepositoryIfNeeded(ctx context.Context, repoPath string) error {
	lockThreshold := time.Now().Add(-1 * setting.Repository.DanglingLockThreshold)
	return cleanLocksIfNeeded(repoPath, lockThreshold)
}

func cleanLocksIfNeeded(repoPath string, threshold time.Time) error {
	if repoPath == "" {
		return nil
	}
	log.Trace("Checking if repository %s is locked [lock threshold is %s]", repoPath, threshold)
	return filepath.Walk(repoPath, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if err := cleanLockIfNeeded(filePath, fileInfo, threshold); err != nil {
			log.Error("Failed to remove lock file %s: %v", filePath, err)
			return err
		}
		return nil
	})
}

func cleanLockIfNeeded(filePath string, fileInfo os.FileInfo, threshold time.Time) error {
	if isLock(fileInfo) {
		if fileInfo.ModTime().Before(threshold) {
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				return err
			}
			log.Info("Lock file %s has been removed since its older than %s [timestamp: %s]", filePath, threshold, fileInfo.ModTime())
			return nil
		}
		log.Warn("Cannot exclude lock file %s because it is younger than the threshold %s [timestamp: %s]", filePath, threshold, fileInfo.ModTime())
		return nil
	}
	return nil
}

func isLock(lockFile os.FileInfo) bool {
	return !lockFile.IsDir() && strings.HasSuffix(lockFile.Name(), ".lock")
}
