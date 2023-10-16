// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/externalaccount"
)

// UpdateMigrationPosterID updates all migrated repositories' issues and comments posterID
func UpdateMigrationPosterID(ctx context.Context) error {
	for _, gitService := range structs.SupportedFullGitService {
		select {
		case <-ctx.Done():
			log.Warn("UpdateMigrationPosterID aborted before %s", gitService.Name())
			return db.ErrCancelledf("during UpdateMigrationPosterID before %s", gitService.Name())
		default:
		}
		if err := updateMigrationPosterIDByGitService(ctx, gitService); err != nil {
			log.Error("updateMigrationPosterIDByGitService failed: %v", err)
		}
	}
	return nil
}

func updateMigrationPosterIDByGitService(ctx context.Context, tp structs.GitServiceType) error {
	provider := tp.Name()
	if len(provider) == 0 {
		return nil
	}

	const batchSize = 100
	var start int
	for {
		select {
		case <-ctx.Done():
			log.Warn("UpdateMigrationPosterIDByGitService(%s) cancelled", tp.Name())
			return nil
		default:
		}

		users, err := user_model.FindExternalUsersByProvider(ctx, user_model.FindExternalUserOptions{
			Provider: provider,
			Start:    start,
			Limit:    batchSize,
		})
		if err != nil {
			return err
		}

		for _, user := range users {
			select {
			case <-ctx.Done():
				log.Warn("UpdateMigrationPosterIDByGitService(%s) cancelled", tp.Name())
				return nil
			default:
			}
			externalUserID := user.ExternalID
			if err := externalaccount.UpdateMigrationsByType(ctx, tp, externalUserID, user.UserID); err != nil {
				log.Error("UpdateMigrationsByType type %s external user id %v to local user id %v failed: %v", tp.Name(), user.ExternalID, user.UserID, err)
			}
		}

		if len(users) < batchSize {
			break
		}
		start += len(users)
	}
	return nil
}
