// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/repository"
)

func init() {
	Register(&Check{
		Title:                      "Garbage collect LFS",
		Name:                       "gc-lfs",
		IsDefault:                  false,
		Run:                        garbageCollectLFSCheck,
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})
}

func garbageCollectLFSCheck(ctx context.Context, logger log.Logger, autofix bool) error {
	if !setting.LFS.StartServer {
		return fmt.Errorf("LFS support is disabled")
	}

	if err := repository.GarbageCollectLFSMetaObjects(ctx, repository.GarbageCollectLFSMetaObjectsOptions{
		LogDetail: logger.Info,
		AutoFix:   autofix,
		// Only attempt to garbage collect lfs meta objects older than a week as the order of git lfs upload
		// and git object upload is not necessarily guaranteed. It's possible to imagine a situation whereby
		// an LFS object is uploaded but the git branch is not uploaded immediately, or there are some rapid
		// changes in new branches that might lead to lfs objects becoming temporarily unassociated with git
		// objects.
		//
		// It is likely that a week is potentially excessive but it should definitely be enough that any
		// unassociated LFS object is genuinely unassociated.
		OlderThan: time.Now().Add(-24 * time.Hour * 7),
		// We don't set the UpdatedLessRecentlyThan because we want to do a full GC
	}); err != nil {
		return err
	}

	return checkStorage(&checkStorageOptions{LFS: true})(ctx, logger, autofix)
}
