// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
)

// SyncExternalUsers is used to synchronize users with external authorization source
func SyncExternalUsers(ctx context.Context, updateExisting bool) error {
	log.Trace("Doing: SyncExternalUsers")

	ls, err := auth.FindSources(ctx, auth.FindSourcesOptions{})
	if err != nil {
		log.Error("SyncExternalUsers: %v", err)
		return err
	}

	for _, s := range ls {
		if !s.IsActive || !s.IsSyncEnabled {
			continue
		}
		select {
		case <-ctx.Done():
			log.Warn("SyncExternalUsers: Cancelled before update of %s", s.Name)
			return db.ErrCancelledf("Before update of %s", s.Name)
		default:
		}

		if syncable, ok := s.Cfg.(SynchronizableSource); ok {
			err := syncable.Sync(ctx, updateExisting)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
