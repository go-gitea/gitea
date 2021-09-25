// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
)

func checkAttachmentStorageFiles(logger log.Logger, autofix bool) error {
	var total, garbageNum int
	var deletePaths []string
	if err := storage.Attachments.IterateObjects(func(p string, obj storage.Object) error {
		defer obj.Close()

		total++
		stat, err := obj.Stat()
		if err != nil {
			return err
		}
		exist, err := models.ExistAttachmentsByUUID(stat.Name())
		if err != nil {
			return err
		}
		if !exist {
			garbageNum++
			if autofix {
				deletePaths = append(deletePaths, p)
			}
		}
		return nil
	}); err != nil {
		logger.Error("storage.Attachments.IterateObjects failed: %v", err)
		return err
	}

	if garbageNum > 0 {
		if autofix {
			var deletedNum int
			for _, p := range deletePaths {
				if err := storage.Attachments.Delete(p); err != nil {
					log.Error("Delete attachment %s failed: %v", p, err)
				} else {
					deletedNum++
				}
			}
			logger.Info("%d missed information attachment detected, %d deleted.", garbageNum, deletedNum)
		} else {
			logger.Warn("Checked %d attachment, %d missed information.", total, garbageNum)
		}
	}
	return nil
}

func checkStorageFiles(logger log.Logger, autofix bool) error {
	if err := storage.Init(); err != nil {
		logger.Error("storage.Init failed: %v", err)
		return err
	}
	return checkAttachmentStorageFiles(logger, autofix)
}

func init() {
	Register(&Check{
		Title:                      "Check if there is garbage storage files",
		Name:                       "storages",
		IsDefault:                  false,
		Run:                        checkStorageFiles,
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})
}
