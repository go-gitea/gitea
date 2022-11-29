// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"

	"code.gitea.io/gitea/modules/log"
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
	if err := repository.GarbageCollectLFSMetaObjects(ctx, logger, autofix); err != nil {
		return err
	}

	return checkStorage(&checkStorageOptions{LFS: true})(ctx, logger, autofix)
}
