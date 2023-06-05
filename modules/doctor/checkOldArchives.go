// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"os"
	"path/filepath"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

func checkOldArchives(ctx context.Context, logger log.Logger, autofix bool) error {
	numRepos := 0
	numReposUpdated := 0
	err := iterateRepositories(ctx, func(repo *repo_model.Repository) error {
		if repo.IsEmpty {
			return nil
		}

		p := filepath.Join(repo.RepoPath(), "archives")
		isDir, err := util.IsDir(p)
		if err != nil {
			log.Warn("check if %s is directory failed: %v", p, err)
		}
		if isDir {
			numRepos++
			if autofix {
				if err := os.RemoveAll(p); err == nil {
					numReposUpdated++
				} else {
					log.Warn("remove %s failed: %v", p, err)
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
