// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
)

// UpdateMigrationPosterID updates all migrated repositories' issues and comments posterID
func UpdateMigrationPosterID() {
	for _, gitService := range structs.SupportedFullGitService {
		if err := updateMigrationPosterIDByGitService(gitService); err != nil {
			log.Error("updateMigrationPosterIDByGitService failed: %v", err)
		}
	}
}

func updateMigrationPosterIDByGitService(tp structs.GitServiceType) error {
	provider := tp.Name()
	if len(provider) == 0 {
		return nil
	}

	const batchSize = 100
	var start int
	for {
		users, err := models.FindExternalUsersByProvider(models.FindExternalUserOptions{
			Provider: provider,
			Start:    start,
			Limit:    batchSize,
		})
		if err != nil {
			return err
		}

		for _, user := range users {
			externalUserID := user.ExternalID
			if err := models.UpdateMigrationsByType(tp, externalUserID, user.UserID); err != nil {
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
