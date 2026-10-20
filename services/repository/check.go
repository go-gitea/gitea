// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	system_model "gitea.dev/models/system"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
	repo_module "gitea.dev/modules/repository"

	"xorm.io/builder"
)

// GitFsckRepos calls 'git fsck' to check repository health.
func GitFsckRepos(ctx context.Context, timeout time.Duration, args gitcmd.TrustedCmdArgs) error {
	log.Trace("Doing: GitFsck")

	if err := db.Iterate(
		ctx,
		builder.Expr("id>0 AND is_fsck_enabled=?", true),
		func(ctx context.Context, repo *repo_model.Repository) error {
			select {
			case <-ctx.Done():
				return db.ErrCancelledf("before fsck of %s", repo.FullName())
			default:
			}
			return GitFsckRepo(ctx, repo, timeout, args)
		},
	); err != nil {
		log.Trace("Error: GitFsck: %v", err)
		return err
	}

	log.Trace("Finished: GitFsck")
	return nil
}

// GitFsckRepo calls 'git fsck' to check an individual repository's health.
func GitFsckRepo(ctx context.Context, repo *repo_model.Repository, timeout time.Duration, args gitcmd.TrustedCmdArgs) error {
	log.Trace("Running health check on repository %-v", repo.FullName())
	if err := git.Fsck(ctx, repo, timeout, args); err != nil {
		log.Warn("Failed to health check repository (%-v): %v", repo.FullName(), err)
		if err = system_model.CreateRepositoryNotice("Failed to health check repository (%s): %v", repo.FullName(), err); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
	return nil
}

func MaintainRepositories(ctx context.Context) error {
	return db.Iterate(ctx, builder.Gt{"id": 0}, maintainRepository)
}

func maintainRepository(ctx context.Context, repo *repo_model.Repository) error {
	if ctx.Err() != nil {
		return db.ErrCancelledf("before maintenance of %s", repo.FullName())
	}
	for _, gitRepo := range []git.RepositoryFacade{repo.WikiStorageRepo(), repo} {
		if exist, err := git.IsRepositoryExist(ctx, gitRepo); !exist {
			if err != nil {
				log.Error("Failed to check %s: %v", gitRepo.LogString(), err)
			}
			continue
		}
		changed, err := git.MaintainRepository(ctx, gitRepo)
		if err != nil {
			log.Error("Failed to maintain %s: %v", gitRepo.LogString(), err)
			if err := system_model.CreateRepositoryNotice("Failed to maintain %s: %v", gitRepo.LogString(), err); err != nil {
				log.Error("CreateRepositoryNotice: %v", err)
			}
		}
		if changed && gitRepo == repo {
			if err := repo_module.UpdateRepoSize(ctx, repo); err != nil {
				log.Error("Failed to update the size of %s: %v", gitRepo.LogString(), err)
				if err := system_model.CreateRepositoryNotice("Failed to update the size of %s: %v", gitRepo.LogString(), err); err != nil {
					log.Error("CreateRepositoryNotice: %v", err)
				}
			}
		}
	}
	return nil
}

func gatherMissingRepoRecords(ctx context.Context) (repo_model.RepositoryList, error) {
	repos := make([]*repo_model.Repository, 0, 10)
	if err := db.Iterate(
		ctx,
		builder.Gt{"id": 0},
		func(ctx context.Context, repo *repo_model.Repository) error {
			select {
			case <-ctx.Done():
				return db.ErrCancelledf("during gathering missing repo records before checking %s", repo.FullName())
			default:
			}
			exist, err := git.IsRepositoryExist(ctx, repo)
			if err != nil {
				return fmt.Errorf("Unable to check dir for %s. %w", repo.FullName(), err)
			}
			if !exist {
				repos = append(repos, repo)
			}
			return nil
		},
	); err != nil {
		if strings.HasPrefix(err.Error(), "Aborted gathering missing repo") {
			return nil, err
		}
		if err2 := system_model.CreateRepositoryNotice("gatherMissingRepoRecords: %v", err); err2 != nil {
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
		if err := DeleteRepositoryDirectly(ctx, repo.ID); err != nil {
			log.Error("Failed to DeleteRepository %-v: Error: %v", repo, err)
			if err2 := system_model.CreateRepositoryNotice("Failed to DeleteRepository (%s) [%d]: Error: %v", repo.FullName(), repo.ID, err); err2 != nil {
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
		if err := git.InitRepository(ctx, repo, repo.ObjectFormatName, repo.DefaultBranch); err != nil {
			log.Error("Unable (re)initialize repository %d at %s. Error: %v", repo.ID, repo.FullName(), err)
			if err2 := system_model.CreateRepositoryNotice("InitRepository (%s) [%d]: %v", repo.FullName(), repo.ID, err); err2 != nil {
				log.Error("CreateRepositoryNotice: %v", err2)
			}
		}
	}
	return nil
}
