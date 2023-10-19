// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// GitFsckRepos calls 'git fsck' to check repository health.
func GitFsckRepos(ctx context.Context, timeout time.Duration, args git.TrustedCmdArgs) error {
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
func GitFsckRepo(ctx context.Context, repo *repo_model.Repository, timeout time.Duration, args git.TrustedCmdArgs) error {
	log.Trace("Running health check on repository %-v", repo)
	repoPath := repo.RepoPath()
	if err := git.Fsck(ctx, repoPath, timeout, args); err != nil {
		log.Warn("Failed to health check repository (%-v): %v", repo, err)
		if err = system_model.CreateRepositoryNotice("Failed to health check repository (%s): %v", repo.FullName(), err); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
	return nil
}

// GitGcRepos calls 'git gc' to remove unnecessary files and optimize the local repository
func GitGcRepos(ctx context.Context, timeout time.Duration, args git.TrustedCmdArgs) error {
	log.Trace("Doing: GitGcRepos")

	if err := db.Iterate(
		ctx,
		builder.Gt{"id": 0},
		func(ctx context.Context, repo *repo_model.Repository) error {
			select {
			case <-ctx.Done():
				return db.ErrCancelledf("before GC of %s", repo.FullName())
			default:
			}
			// we can ignore the error here because it will be logged in GitGCRepo
			_ = GitGcRepo(ctx, repo, timeout, args)
			return nil
		},
	); err != nil {
		return err
	}

	log.Trace("Finished: GitGcRepos")
	return nil
}

// GitGcRepo calls 'git gc' to remove unnecessary files and optimize the local repository
func GitGcRepo(ctx context.Context, repo *repo_model.Repository, timeout time.Duration, args git.TrustedCmdArgs) error {
	log.Trace("Running git gc on %-v", repo)
	command := git.NewCommand(ctx, "gc").AddArguments(args...).
		SetDescription(fmt.Sprintf("Repository Garbage Collection: %s", repo.FullName()))
	var stdout string
	var err error
	stdout, _, err = command.RunStdString(&git.RunOpts{Timeout: timeout, Dir: repo.RepoPath()})

	if err != nil {
		log.Error("Repository garbage collection failed for %-v. Stdout: %s\nError: %v", repo, stdout, err)
		desc := fmt.Sprintf("Repository garbage collection failed for %s. Stdout: %s\nError: %v", repo.RepoPath(), stdout, err)
		if err := system_model.CreateRepositoryNotice(desc); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
		return fmt.Errorf("Repository garbage collection failed in repo: %s: Error: %w", repo.FullName(), err)
	}

	// Now update the size of the repository
	if err := repo_module.UpdateRepoSize(ctx, repo); err != nil {
		log.Error("Updating size as part of garbage collection failed for %-v. Stdout: %s\nError: %v", repo, stdout, err)
		desc := fmt.Sprintf("Updating size as part of garbage collection failed for %s. Stdout: %s\nError: %v", repo.RepoPath(), stdout, err)
		if err := system_model.CreateRepositoryNotice(desc); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
		return fmt.Errorf("Updating size as part of garbage collection failed in repo: %s: Error: %w", repo.FullName(), err)
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
		if err := DeleteRepositoryDirectly(ctx, doer, repo.ID); err != nil {
			log.Error("Failed to DeleteRepository %-v: Error: %v", repo, err)
			if err2 := system_model.CreateRepositoryNotice("Failed to DeleteRepository %s [%d]: Error: %v", repo.FullName(), repo.ID, err); err2 != nil {
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
			if err2 := system_model.CreateRepositoryNotice("InitRepository [%d]: %v", repo.ID, err); err2 != nil {
				log.Error("CreateRepositoryNotice: %v", err2)
			}
		}
	}
	return nil
}
