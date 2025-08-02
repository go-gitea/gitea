// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"errors"
	"fmt"
	"os"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
)

func ProcessDeletions(ctx context.Context, deletionIDs []int64) {
	for _, deletionID := range deletionIDs {
		deletion, exist, err := db.GetByID[system.StoragePathDeletion](ctx, deletionID)
		if err != nil {
			log.Error("Failed to get deletion by ID %d: %v", deletionID, err)
			continue
		}
		if !exist {
			continue
		}

		theStorage, err := storage.GetStorageByName(deletion.StorageName)
		if err != nil {
			log.Error("Failed to get storage by name %s: %v", deletion.StorageName, err)
			continue
		}
		if err := theStorage.Delete(deletion.RelativePath); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				log.Error("delete pending deletion[relative path: %s] failed: %v", deletion.RelativePath, err)
				if deletion.DeleteFailedCount%3 == 0 {
					_ = system.CreateNotice(ctx, system.NoticeRepository, fmt.Sprintf("Failed to delete pending deletion %s (%d times): %v", deletion.RelativePath, deletion.DeleteFailedCount+1, err))
				}
				if err := system.UpdateDeletionFailure(ctx, deletion, err); err != nil {
					log.Error("Failed to update deletion failure for ID %d: %v", deletion.ID, err)
				}
				continue
			}
		}
		if _, err := db.DeleteByID[system.StoragePathDeletion](ctx, deletion.ID); err != nil {
			log.Error("Failed to delete pending deletion by ID %d(will be tried later): %v", deletion.ID, err)
		} else {
			log.Trace("Pending deletion %s deleted from database", deletion.RelativePath)
		}
	}
}

// ScanToBeDeletedFilesOrDir scans for files or directories that are marked as to
// be deleted and processes them in batches.
func ScanToBeDeletedFilesOrDir(ctx context.Context) error {
	deletionIDs := make([]int64, 0, 100)
	lastID := int64(0)
	for {
		if err := db.GetEngine(ctx).
			Select("id").
			Where("id > ?", lastID).
			Asc("id").
			Limit(100).
			Find(&deletionIDs); err != nil {
			return fmt.Errorf("scan to-be-deleted files or directories: %w", err)
		}

		if len(deletionIDs) == 0 {
			log.Trace("No more files or directories to be deleted")
			break
		}
		ProcessDeletions(ctx, deletionIDs)

		lastID = deletionIDs[len(deletionIDs)-1]
		deletionIDs = deletionIDs[0:0]
	}

	return nil
}
