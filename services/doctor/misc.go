// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"

	lru "github.com/hashicorp/golang-lru/v2"
	"xorm.io/builder"
)

func iterateRepositories(ctx context.Context, each func(*repo_model.Repository) error) error {
	err := db.Iterate(
		ctx,
		builder.Gt{"id": 0},
		func(ctx context.Context, bean *repo_model.Repository) error {
			return each(bean)
		},
	)
	return err
}

func checkHooks(ctx context.Context, logger log.Logger, autofix bool) error {
	if err := iterateRepositories(ctx, func(repo *repo_model.Repository) error {
		results, err := gitrepo.CheckDelegateHooks(ctx, repo)
		if err != nil {
			logger.Critical("Unable to check delegate hooks for repo %-v. ERROR: %v", repo, err)
			return fmt.Errorf("Unable to check delegate hooks for repo %-v. ERROR: %w", repo, err)
		}
		if len(results) > 0 && autofix {
			logger.Warn("Regenerated hooks for %s", repo.FullName())
			if err := gitrepo.CreateDelegateHooks(ctx, repo); err != nil {
				logger.Critical("Unable to recreate delegate hooks for %-v. ERROR: %v", repo, err)
				return fmt.Errorf("Unable to recreate delegate hooks for %-v. ERROR: %w", repo, err)
			}
		}
		for _, result := range results {
			logger.Warn(result)
		}
		return nil
	}); err != nil {
		logger.Critical("Errors noted whilst checking delegate hooks.")
		return err
	}
	return nil
}

func checkUserStarNum(ctx context.Context, logger log.Logger, autofix bool) error {
	if autofix {
		if err := models.DoctorUserStarNum(ctx); err != nil {
			logger.Critical("Unable update User Stars numbers")
			return err
		}
		logger.Info("Updated User Stars numbers.")
	} else {
		logger.Info("No check available for User Stars numbers (skipped)")
	}
	return nil
}

func checkDaemonExport(ctx context.Context, logger log.Logger, autofix bool) error {
	numRepos := 0
	numNeedUpdate := 0
	cache, err := lru.New[int64, any](512)
	if err != nil {
		logger.Critical("Unable to create cache: %v", err)
		return err
	}
	if err := iterateRepositories(ctx, func(repo *repo_model.Repository) error {
		numRepos++

		if owner, has := cache.Get(repo.OwnerID); has {
			repo.Owner = owner.(*user_model.User)
		} else {
			if err := repo.LoadOwner(ctx); err != nil {
				return err
			}
			cache.Add(repo.OwnerID, repo.Owner)
		}

		// Create/Remove git-daemon-export-ok for git-daemon...
		daemonExportFile := `git-daemon-export-ok`
		isExist, err := gitrepo.IsRepoFileExist(ctx, repo, daemonExportFile)
		if err != nil {
			log.Error("Unable to check if %s:%s exists. Error: %v", repo.FullName(), daemonExportFile, err)
			return err
		}
		isPublic := !repo.IsPrivate && repo.Owner.Visibility == structs.VisibleTypePublic

		if isPublic != isExist {
			numNeedUpdate++
			if autofix {
				if !isPublic && isExist {
					if err = gitrepo.RemoveRepoFileOrDir(ctx, repo, daemonExportFile); err != nil {
						log.Error("Failed to remove %s:%s: %v", repo.FullName(), daemonExportFile, err)
					}
				} else if isPublic && !isExist {
					if f, err := gitrepo.CreateRepoFile(ctx, repo, daemonExportFile); err != nil {
						log.Error("Failed to create %s:%s: %v", repo.FullName(), daemonExportFile, err)
					} else {
						f.Close()
					}
				}
			}
		}
		return nil
	}); err != nil {
		logger.Critical("Unable to checkDaemonExport: %v", err)
		return err
	}

	if autofix {
		logger.Info("Updated git-daemon-export-ok files for %d of %d repositories.", numNeedUpdate, numRepos)
	} else {
		logger.Info("Checked %d repositories, %d need updates.", numRepos, numNeedUpdate)
	}

	return nil
}

func checkCommitGraph(ctx context.Context, logger log.Logger, autofix bool) error {
	numRepos := 0
	numNeedUpdate := 0
	numWritten := 0
	if err := iterateRepositories(ctx, func(repo *repo_model.Repository) error {
		numRepos++

		commitGraphExists := func() (bool, error) {
			// Check commit-graph exists
			commitGraphFile := `objects/info/commit-graph`
			isExist, err := gitrepo.IsRepoFileExist(ctx, repo, commitGraphFile)
			if err != nil {
				logger.Error("Unable to check if %s exists. Error: %v", commitGraphFile, err)
				return false, err
			}

			if !isExist {
				commitGraphsDir := `objects/info/commit-graphs`
				isExist, err = gitrepo.IsRepoDirExist(ctx, repo, commitGraphsDir)
				if err != nil {
					logger.Error("Unable to check if %s exists. Error: %v", commitGraphsDir, err)
					return false, err
				}
			}
			return isExist, nil
		}

		isExist, err := commitGraphExists()
		if err != nil {
			return err
		}
		if !isExist {
			numNeedUpdate++
			if autofix {
				if err := gitrepo.WriteCommitGraph(ctx, repo); err != nil {
					logger.Error("Unable to write commit-graph in %s. Error: %v", repo.FullName(), err)
					return err
				}
				isExist, err := commitGraphExists()
				if err != nil {
					return err
				}
				if isExist {
					numWritten++
					logger.Info("Commit-graph written:    %s", repo.FullName())
				} else {
					logger.Warn("No commit-graph written: %s", repo.FullName())
				}
			}
		}
		return nil
	}); err != nil {
		logger.Critical("Unable to checkCommitGraph: %v", err)
		return err
	}

	if autofix {
		logger.Info("Wrote commit-graph files for %d of %d repositories.", numWritten, numRepos)
	} else {
		logger.Info("Checked %d repositories, %d without commit-graphs.", numRepos, numNeedUpdate)
	}

	return nil
}

func init() {
	Register(&Check{
		Title:     "Check if hook files are up-to-date and executable",
		Name:      "hooks",
		IsDefault: false,
		Run:       checkHooks,
		Priority:  6,
	})
	Register(&Check{
		Title:     "Recalculate Stars number for all user",
		Name:      "recalculate-stars-number",
		IsDefault: false,
		Run:       checkUserStarNum,
		Priority:  6,
	})
	Register(&Check{
		Title:     "Check git-daemon-export-ok files",
		Name:      "check-git-daemon-export-ok",
		IsDefault: false,
		Run:       checkDaemonExport,
		Priority:  8,
	})
	Register(&Check{
		Title:     "Check commit-graphs",
		Name:      "check-commit-graphs",
		IsDefault: false,
		Run:       checkCommitGraph,
		Priority:  9,
	})
}
