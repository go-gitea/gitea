// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/unknwon/com"
	"xorm.io/builder"
)

// GitFsck calls 'git fsck' to check repository health.
func GitFsck(ctx context.Context) error {
	log.Trace("Doing: GitFsck")

	if err := models.Iterate(
		models.DefaultDBContext(),
		new(models.Repository),
		builder.Expr("id>0 AND is_fsck_enabled=?", true),
		func(idx int, bean interface{}) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("Aborted due to shutdown")
			default:
			}
			repo := bean.(*models.Repository)
			repoPath := repo.RepoPath()
			log.Trace("Running health check on repository %s", repoPath)
			if err := git.Fsck(repoPath, setting.Cron.RepoHealthCheck.Timeout, setting.Cron.RepoHealthCheck.Args...); err != nil {
				desc := fmt.Sprintf("Failed to health check repository (%s): %v", repoPath, err)
				log.Warn(desc)
				if err = models.CreateRepositoryNotice(desc); err != nil {
					log.Error("CreateRepositoryNotice: %v", err)
				}
			}
			return nil
		},
	); err != nil {
		return err
	}

	log.Trace("Finished: GitFsck")
	return nil
}

// GitGcRepos calls 'git gc' to remove unnecessary files and optimize the local repository
func GitGcRepos(ctx context.Context) error {
	log.Trace("Doing: GitGcRepos")
	args := append([]string{"gc"}, setting.Git.GCArgs...)

	if err := models.Iterate(
		models.DefaultDBContext(),
		new(models.Repository),
		builder.Gt{"id": 0},
		func(idx int, bean interface{}) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("Aborted due to shutdown")
			default:
			}

			repo := bean.(*models.Repository)
			if err := repo.GetOwner(); err != nil {
				return err
			}
			if stdout, err := git.NewCommand(args...).
				SetDescription(fmt.Sprintf("Repository Garbage Collection: %s", repo.FullName())).
				RunInDirTimeout(
					time.Duration(setting.Git.Timeout.GC)*time.Second,
					repo.RepoPath()); err != nil {
				log.Error("Repository garbage collection failed for %v. Stdout: %s\nError: %v", repo, stdout, err)
				return fmt.Errorf("Repository garbage collection failed: Error: %v", err)
			}
			return nil
		},
	); err != nil {
		return err
	}

	log.Trace("Finished: GitGcRepos")
	return nil
}

func gatherMissingRepoRecords() ([]*models.Repository, error) {
	repos := make([]*models.Repository, 0, 10)
	if err := models.Iterate(
		models.DefaultDBContext(),
		new(models.Repository),
		builder.Gt{"id": 0},
		func(idx int, bean interface{}) error {
			repo := bean.(*models.Repository)
			if !com.IsDir(repo.RepoPath()) {
				repos = append(repos, repo)
			}
			return nil
		},
	); err != nil {
		if err2 := models.CreateRepositoryNotice(fmt.Sprintf("gatherMissingRepoRecords: %v", err)); err2 != nil {
			return nil, fmt.Errorf("CreateRepositoryNotice: %v", err)
		}
	}
	return repos, nil
}

// DeleteMissingRepositories deletes all repository records that lost Git files.
func DeleteMissingRepositories(doer *models.User) error {
	repos, err := gatherMissingRepoRecords()
	if err != nil {
		return fmt.Errorf("gatherMissingRepoRecords: %v", err)
	}

	if len(repos) == 0 {
		return nil
	}

	for _, repo := range repos {
		log.Trace("Deleting %d/%d...", repo.OwnerID, repo.ID)
		if err := models.DeleteRepository(doer, repo.OwnerID, repo.ID); err != nil {
			if err2 := models.CreateRepositoryNotice(fmt.Sprintf("DeleteRepository [%d]: %v", repo.ID, err)); err2 != nil {
				return fmt.Errorf("CreateRepositoryNotice: %v", err)
			}
		}
	}
	return nil
}

// ReinitMissingRepositories reinitializes all repository records that lost Git files.
func ReinitMissingRepositories() error {
	repos, err := gatherMissingRepoRecords()
	if err != nil {
		return fmt.Errorf("gatherMissingRepoRecords: %v", err)
	}

	if len(repos) == 0 {
		return nil
	}

	for _, repo := range repos {
		log.Trace("Initializing %d/%d...", repo.OwnerID, repo.ID)
		if err := git.InitRepository(repo.RepoPath(), true); err != nil {
			if err2 := models.CreateRepositoryNotice(fmt.Sprintf("InitRepository [%d]: %v", repo.ID, err)); err2 != nil {
				return fmt.Errorf("CreateRepositoryNotice: %v", err)
			}
		}
	}
	return nil
}
