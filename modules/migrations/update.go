// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"strconv"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
)

// UpdateRepoMigrations will update posterid on issues/comments/prs when external login user
// login or when migrating
func UpdateRepoMigrations(repoID, externalUserID, userID int64) error {
	if err := models.UpdateIssuesMigrations(repoID, externalUserID, userID); err != nil {
		return err
	}

	return models.UpdateCommentsMigrations(repoID, externalUserID, userID)
}

// UpdateMigrationPosterID updates all migrated repositories' issues and comments posterID
func UpdateMigrationPosterID() {
	if err := updateMigrationPosterIDByGitService(structs.GithubService); err != nil {
		log.Error("updateMigrationPosterIDByGitService failed: %v", err)
	}
}

func updateMigrationPosterIDByGitService(tp structs.GitServiceType) error {
	provider := tp.Name()
	if len(provider) == 0 {
		return nil
	}

	var repoStart int
	const batchSize = 100
	for {
		ids, err := models.FindMigratedRepositoryIDs(tp, batchSize, repoStart)
		if err != nil {
			return err
		}

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
				externalUserID, err := strconv.ParseInt(user.ExternalID, 10, 64)
				if err != nil {
					log.Warn("Parse externalUser %#v 's userID failed: %v", user, err)
					continue
				}
				for _, id := range ids {
					if err = UpdateRepoMigrations(id, externalUserID, user.UserID); err != nil {
						log.Error("UpdateRepoMigrations repo %v, github user id %v, user id %v failed: %v", id, user.ExternalID, user.UserID, err)
					}
				}
			}

			if len(users) < batchSize {
				break
			}
			start += len(users)
		}

		if len(ids) < batchSize {
			return nil
		}
		repoStart += len(ids)
	}
}
