// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	admin_model "code.gitea.io/gitea/models/admin"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// GitFsck calls 'git fsck' to check repository health.
func GitFsck(ctx context.Context, timeout time.Duration, args []string) error {
	log.Trace("Doing: GitFsck")

	if err := db.Iterate(
		db.DefaultContext,
		new(repo_model.Repository),
		builder.Expr("id>0 AND is_fsck_enabled=?", true),
		func(idx int, bean interface{}) error {
			repo := bean.(*repo_model.Repository)
			select {
			case <-ctx.Done():
				return db.ErrCancelledf("before fsck of %s", repo.FullName())
			default:
			}
			log.Trace("Running health check on repository %v", repo)
			repoPath := repo.RepoPath()
			if err := git.Fsck(ctx, repoPath, timeout, args...); err != nil {
				log.Warn("Failed to health check repository (%v): %v", repo, err)
				if err = admin_model.CreateRepositoryNotice("Failed to health check repository (%s): %v", repo.FullName(), err); err != nil {
					log.Error("CreateRepositoryNotice: %v", err)
				}
			}
			return nil
		},
	); err != nil {
		log.Trace("Error: GitFsck: %v", err)
		return err
	}

	log.Trace("Finished: GitFsck")
	return nil
}

// GitGcRepos calls 'git gc' to remove unnecessary files and optimize the local repository
func GitGcRepos(ctx context.Context, timeout time.Duration, args ...string) error {
	log.Trace("Doing: GitGcRepos")
	args = append([]string{"gc"}, args...)

	if err := db.Iterate(
		db.DefaultContext,
		new(repo_model.Repository),
		builder.Gt{"id": 0},
		func(idx int, bean interface{}) error {
			repo := bean.(*repo_model.Repository)
			select {
			case <-ctx.Done():
				return db.ErrCancelledf("before GC of %s", repo.FullName())
			default:
			}
			log.Trace("Running git gc on %v", repo)
			command := git.NewCommand(ctx, args...).
				SetDescription(fmt.Sprintf("Repository Garbage Collection: %s", repo.FullName()))
			var stdout string
			var err error
			if timeout > 0 {
				var stdoutBytes []byte
				stdoutBytes, err = command.RunInDirTimeout(
					timeout,
					repo.RepoPath())
				stdout = string(stdoutBytes)
			} else {
				stdout, err = command.RunInDir(repo.RepoPath())
			}

			if err != nil {
				log.Error("Repository garbage collection failed for %v. Stdout: %s\nError: %v", repo, stdout, err)
				desc := fmt.Sprintf("Repository garbage collection failed for %s. Stdout: %s\nError: %v", repo.RepoPath(), stdout, err)
				if err = admin_model.CreateRepositoryNotice(desc); err != nil {
					log.Error("CreateRepositoryNotice: %v", err)
				}
				return fmt.Errorf("Repository garbage collection failed in repo: %s: Error: %v", repo.FullName(), err)
			}

			// Now update the size of the repository
			if err := models.UpdateRepoSize(db.DefaultContext, repo); err != nil {
				log.Error("Updating size as part of garbage collection failed for %v. Stdout: %s\nError: %v", repo, stdout, err)
				desc := fmt.Sprintf("Updating size as part of garbage collection failed for %s. Stdout: %s\nError: %v", repo.RepoPath(), stdout, err)
				if err = admin_model.CreateRepositoryNotice(desc); err != nil {
					log.Error("CreateRepositoryNotice: %v", err)
				}
				return fmt.Errorf("Updating size as part of garbage collection failed in repo: %s: Error: %v", repo.FullName(), err)
			}

			return nil
		},
	); err != nil {
		return err
	}

	log.Trace("Finished: GitGcRepos")
	return nil
}

func gatherMissingRepoRecords(ctx context.Context) ([]*repo_model.Repository, error) {
	repos := make([]*repo_model.Repository, 0, 10)
	if err := db.Iterate(
		db.DefaultContext,
		new(repo_model.Repository),
		builder.Gt{"id": 0},
		func(idx int, bean interface{}) error {
			repo := bean.(*repo_model.Repository)
			select {
			case <-ctx.Done():
				return db.ErrCancelledf("during gathering missing repo records before checking %s", repo.FullName())
			default:
			}
			isDir, err := util.IsDir(repo.RepoPath())
			if err != nil {
				return fmt.Errorf("Unable to check dir for %s. %w", repo.FullName(), err)
			}
			if !isDir {
				repos = append(repos, repo)
			}
			return nil
		},
	); err != nil {
		if strings.HasPrefix(err.Error(), "Aborted gathering missing repo") {
			return nil, err
		}
		if err2 := admin_model.CreateRepositoryNotice("gatherMissingRepoRecords: %v", err); err2 != nil {
			log.Error("CreateRepositoryNotice: %v", err2)
		}
		return nil, err
	}
	return repos, nil
}

// DeleteMissingRepositories deletes all repository records that lost Git files.
func DeleteMissingRepositories(ctx context.Context, doer *user_model.User) error {
	repos, err := gatherMissingRepoRecords(ctx)
	if err != nil {
		return err
	}

	if len(repos) == 0 {
		return nil
	}

	for _, repo := range repos {
		select {
		case <-ctx.Done():
			return db.ErrCancelledf("during DeleteMissingRepositories before %s", repo.FullName())
		default:
		}
		log.Trace("Deleting %d/%d...", repo.OwnerID, repo.ID)
		if err := models.DeleteRepository(doer, repo.OwnerID, repo.ID); err != nil {
			log.Error("Failed to DeleteRepository %s [%d]: Error: %v", repo.FullName(), repo.ID, err)
			if err2 := admin_model.CreateRepositoryNotice("Failed to DeleteRepository %s [%d]: Error: %v", repo.FullName(), repo.ID, err); err2 != nil {
				log.Error("CreateRepositoryNotice: %v", err)
			}
		}
	}
	return nil
}

// ReinitMissingRepositories reinitializes all repository records that lost Git files.
func ReinitMissingRepositories(ctx context.Context) error {
	repos, err := gatherMissingRepoRecords(ctx)
	if err != nil {
		return err
	}

	if len(repos) == 0 {
		return nil
	}

	for _, repo := range repos {
		select {
		case <-ctx.Done():
			return db.ErrCancelledf("during ReinitMissingRepositories before %s", repo.FullName())
		default:
		}
		log.Trace("Initializing %d/%d...", repo.OwnerID, repo.ID)
		if err := git.InitRepository(ctx, repo.RepoPath(), true); err != nil {
			log.Error("Unable (re)initialize repository %d at %s. Error: %v", repo.ID, repo.RepoPath(), err)
			if err2 := admin_model.CreateRepositoryNotice("InitRepository [%d]: %v", repo.ID, err); err2 != nil {
				log.Error("CreateRepositoryNotice: %v", err2)
			}
		}
	}
	return nil
}
