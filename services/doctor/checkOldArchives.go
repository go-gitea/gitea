// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
)

func checkOldArchives(ctx context.Context, logger log.Logger, autofix bool) error {
	numRepos := 0
	numReposUpdated := 0
	err := iterateRepositories(ctx, func(repo *repo_model.Repository) error {
		if repo.IsEmpty {
			return nil
		}

		isDir, err := gitrepo.IsRepoDirExist(ctx, repo, "archives")
		if err != nil {
			log.Warn("check if %s is directory failed: %v", repo.FullName(), err)
		}
		if isDir {
			numRepos++
			if autofix {
				err := gitrepo.RemoveRepoFileOrDir(ctx, repo, "archives")
				if err == nil {
					numReposUpdated++
				} else {
					log.Warn("remove %s failed: %v", repo.FullName(), err)
				}
			}
		}
		return nil
	})

	if autofix {
		logger.Info("%d / %d old archives in repository deleted", numReposUpdated, numRepos)
	} else {
		logger.Info("%d old archives in repository need to be deleted", numRepos)
	}

	return err
}

func init() {
	Register(&Check{
		Title:     "Check old archives",
		Name:      "check-old-archives",
		IsDefault: false,
		Run:       checkOldArchives,
		Priority:  7,
	})
}
