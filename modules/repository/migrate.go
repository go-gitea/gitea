// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// MigrateRepositoryGitData starts migrating git related data after created migrating repository
func MigrateRepositoryGitData(ctx context.Context, u *user_model.User,
	repo *repo_model.Repository, opts migration.MigrateOptions,
	httpTransport *http.Transport,
) (*repo_model.Repository, error) {
	repoPath := repo_model.RepoPath(u.Name, opts.RepoName)

	if u.IsOrganization() {
		t, err := organization.OrgFromUser(u).GetOwnerTeam(ctx)
		if err != nil {
			return nil, err
		}
		repo.NumWatches = t.NumMembers
	} else {
		repo.NumWatches = 1
	}

	migrateTimeout := time.Duration(setting.Git.Timeout.Migrate) * time.Second

	var err error
	if err = util.RemoveAll(repoPath); err != nil {
		return repo, fmt.Errorf("Failed to remove %s: %w", repoPath, err)
	}

	if err = git.Clone(ctx, opts.CloneAddr, repoPath, git.CloneRepoOptions{
		Mirror:        true,
		Quiet:         true,
		Timeout:       migrateTimeout,
		SkipTLSVerify: setting.Migrations.SkipTLSVerify,
	}); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return repo, fmt.Errorf("Clone timed out. Consider increasing [git.timeout] MIGRATE in app.ini. Underlying Error: %w", err)
		}
		return repo, fmt.Errorf("Clone: %w", err)
	}

	if err := git.WriteCommitGraph(ctx, repoPath); err != nil {
		return repo, err
	}

	if opts.Wiki {
		wikiPath := repo_model.WikiPath(u.Name, opts.RepoName)
		wikiRemotePath := WikiRemoteURL(ctx, opts.CloneAddr)
		if len(wikiRemotePath) > 0 {
			if err := util.RemoveAll(wikiPath); err != nil {
				return repo, fmt.Errorf("Failed to remove %s: %w", wikiPath, err)
			}

			if err := git.Clone(ctx, wikiRemotePath, wikiPath, git.CloneRepoOptions{
				Mirror:        true,
				Quiet:         true,
				Timeout:       migrateTimeout,
				Branch:        "master",
				SkipTLSVerify: setting.Migrations.SkipTLSVerify,
			}); err != nil {
				log.Warn("Clone wiki: %v", err)
				if err := util.RemoveAll(wikiPath); err != nil {
					return repo, fmt.Errorf("Failed to remove %s: %w", wikiPath, err)
				}
			} else {
				if err := git.WriteCommitGraph(ctx, wikiPath); err != nil {
					return repo, err
				}
			}
		}
	}

	if repo.OwnerID == u.ID {
		repo.Owner = u
	}

	if err = CheckDaemonExportOK(ctx, repo); err != nil {
		return repo, fmt.Errorf("checkDaemonExportOK: %w", err)
	}

	if stdout, _, err := git.NewCommand(ctx, "update-server-info").
		SetDescription(fmt.Sprintf("MigrateRepositoryGitData(git update-server-info): %s", repoPath)).
		RunStdString(&git.RunOpts{Dir: repoPath}); err != nil {
		log.Error("MigrateRepositoryGitData(git update-server-info) in %v: Stdout: %s\nError: %v", repo, stdout, err)
		return repo, fmt.Errorf("error in MigrateRepositoryGitData(git update-server-info): %w", err)
	}

	gitRepo, err := git.OpenRepository(ctx, repoPath)
	if err != nil {
		return repo, fmt.Errorf("OpenRepository: %w", err)
	}
	defer gitRepo.Close()

	repo.IsEmpty, err = gitRepo.IsEmpty()
	if err != nil {
		return repo, fmt.Errorf("git.IsEmpty: %w", err)
	}

	if !repo.IsEmpty {
		if len(repo.DefaultBranch) == 0 {
			// Try to get HEAD branch and set it as default branch.
			headBranch, err := gitRepo.GetHEADBranch()
			if err != nil {
				return repo, fmt.Errorf("GetHEADBranch: %w", err)
			}
			if headBranch != nil {
				repo.DefaultBranch = headBranch.Name
			}
		}

		if _, err := SyncRepoBranchesWithRepo(ctx, repo, gitRepo, u.ID); err != nil {
			return repo, fmt.Errorf("SyncRepoBranchesWithRepo: %v", err)
		}

		if !opts.Releases {
			// note: this will greatly improve release (tag) sync
			// for pull-mirrors with many tags
			repo.IsMirror = opts.Mirror
			if err = SyncReleasesWithTags(ctx, repo, gitRepo); err != nil {
				log.Error("Failed to synchronize tags to releases for repository: %v", err)
			}
		}

		if opts.LFS {
			endpoint := lfs.DetermineEndpoint(opts.CloneAddr, opts.LFSEndpoint)
			lfsClient := lfs.NewClient(endpoint, httpTransport)
			if err = StoreMissingLfsObjectsInRepository(ctx, repo, gitRepo, lfsClient); err != nil {
				log.Error("Failed to store missing LFS objects for repository: %v", err)
			}
		}
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	if opts.Mirror {
		remoteAddress, err := util.SanitizeURL(opts.CloneAddr)
		if err != nil {
			return repo, err
		}
		mirrorModel := repo_model.Mirror{
			RepoID:         repo.ID,
			Interval:       setting.Mirror.DefaultInterval,
			EnablePrune:    true,
			NextUpdateUnix: timeutil.TimeStampNow().AddDuration(setting.Mirror.DefaultInterval),
			LFS:            opts.LFS,
			RemoteAddress:  remoteAddress,
		}
		if opts.LFS {
			mirrorModel.LFSEndpoint = opts.LFSEndpoint
		}

		if opts.MirrorInterval != "" {
			parsedInterval, err := time.ParseDuration(opts.MirrorInterval)
			if err != nil {
				log.Error("Failed to set Interval: %v", err)
				return repo, err
			}
			if parsedInterval == 0 {
				mirrorModel.Interval = 0
				mirrorModel.NextUpdateUnix = 0
			} else if parsedInterval < setting.Mirror.MinInterval {
				err := fmt.Errorf("interval %s is set below Minimum Interval of %s", parsedInterval, setting.Mirror.MinInterval)
				log.Error("Interval: %s is too frequent", opts.MirrorInterval)
				return repo, err
			} else {
				mirrorModel.Interval = parsedInterval
				mirrorModel.NextUpdateUnix = timeutil.TimeStampNow().AddDuration(parsedInterval)
			}
		}

		if err = repo_model.InsertMirror(ctx, &mirrorModel); err != nil {
			return repo, fmt.Errorf("InsertOne: %w", err)
		}

		repo.IsMirror = true
		if err = UpdateRepository(ctx, repo, false); err != nil {
			return nil, err
		}

		// this is necessary for sync local tags from remote
		configName := fmt.Sprintf("remote.%s.fetch", mirrorModel.GetRemoteName())
		if stdout, _, err := git.NewCommand(ctx, "config").
			AddOptionValues("--add", configName, `+refs/tags/*:refs/tags/*`).
			RunStdString(&git.RunOpts{Dir: repoPath}); err != nil {
			log.Error("MigrateRepositoryGitData(git config --add <remote> +refs/tags/*:refs/tags/*) in %v: Stdout: %s\nError: %v", repo, stdout, err)
			return repo, fmt.Errorf("error in MigrateRepositoryGitData(git config --add <remote> +refs/tags/*:refs/tags/*): %w", err)
		}
	} else {
		if err = UpdateRepoSize(ctx, repo); err != nil {
			log.Error("Failed to update size for repository: %v", err)
		}
		if repo, err = CleanUpMigrateInfo(ctx, repo); err != nil {
			return nil, err
		}
	}

	return repo, committer.Commit()
}

// cleanUpMigrateGitConfig removes mirror info which prevents "push --all".
// This also removes possible user credentials.
func cleanUpMigrateGitConfig(ctx context.Context, repoPath string) error {
	cmd := git.NewCommand(ctx, "remote", "rm", "origin")
	// if the origin does not exist
	_, stderr, err := cmd.RunStdString(&git.RunOpts{
		Dir: repoPath,
	})
	if err != nil && !strings.HasPrefix(stderr, "fatal: No such remote") {
		return err
	}
	return nil
}

// CleanUpMigrateInfo finishes migrating repository and/or wiki with things that don't need to be done for mirrors.
func CleanUpMigrateInfo(ctx context.Context, repo *repo_model.Repository) (*repo_model.Repository, error) {
	repoPath := repo.RepoPath()
	if err := CreateDelegateHooks(repoPath); err != nil {
		return repo, fmt.Errorf("createDelegateHooks: %w", err)
	}
	if repo.HasWiki() {
		if err := CreateDelegateHooks(repo.WikiPath()); err != nil {
			return repo, fmt.Errorf("createDelegateHooks.(wiki): %w", err)
		}
	}

	_, _, err := git.NewCommand(ctx, "remote", "rm", "origin").RunStdString(&git.RunOpts{Dir: repoPath})
	if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
		return repo, fmt.Errorf("CleanUpMigrateInfo: %w", err)
	}

	if repo.HasWiki() {
		if err := cleanUpMigrateGitConfig(ctx, repo.WikiPath()); err != nil {
			return repo, fmt.Errorf("cleanUpMigrateGitConfig (wiki): %w", err)
		}
	}

	return repo, UpdateRepository(ctx, repo, false)
}

// StoreMissingLfsObjectsInRepository downloads missing LFS objects
func StoreMissingLfsObjectsInRepository(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, lfsClient lfs.Client) error {
	contentStore := lfs.NewContentStore()

	pointerChan := make(chan lfs.PointerBlob)
	errChan := make(chan error, 1)
	go lfs.SearchPointerBlobs(ctx, gitRepo, pointerChan, errChan)

	downloadObjects := func(pointers []lfs.Pointer) error {
		err := lfsClient.Download(ctx, pointers, func(p lfs.Pointer, content io.ReadCloser, objectError error) error {
			if objectError != nil {
				return objectError
			}

			defer content.Close()

			_, err := git_model.NewLFSMetaObject(ctx, repo.ID, p)
			if err != nil {
				log.Error("Repo[%-v]: Error creating LFS meta object %-v: %v", repo, p, err)
				return err
			}

			if err := contentStore.Put(p, content); err != nil {
				log.Error("Repo[%-v]: Error storing content for LFS meta object %-v: %v", repo, p, err)
				if _, err2 := git_model.RemoveLFSMetaObjectByOid(ctx, repo.ID, p.Oid); err2 != nil {
					log.Error("Repo[%-v]: Error removing LFS meta object %-v: %v", repo, p, err2)
				}
				return err
			}
			return nil
		})
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
		}
		return err
	}

	var batch []lfs.Pointer
	for pointerBlob := range pointerChan {
		meta, err := git_model.GetLFSMetaObjectByOid(ctx, repo.ID, pointerBlob.Oid)
		if err != nil && err != git_model.ErrLFSObjectNotExist {
			log.Error("Repo[%-v]: Error querying LFS meta object %-v: %v", repo, pointerBlob.Pointer, err)
			return err
		}
		if meta != nil {
			log.Trace("Repo[%-v]: Skipping unknown LFS meta object %-v", repo, pointerBlob.Pointer)
			continue
		}

		log.Trace("Repo[%-v]: LFS object %-v not present in repository", repo, pointerBlob.Pointer)

		exist, err := contentStore.Exists(pointerBlob.Pointer)
		if err != nil {
			log.Error("Repo[%-v]: Error checking if LFS object %-v exists: %v", repo, pointerBlob.Pointer, err)
			return err
		}

		if exist {
			log.Trace("Repo[%-v]: LFS object %-v already present; creating meta object", repo, pointerBlob.Pointer)
			_, err := git_model.NewLFSMetaObject(ctx, repo.ID, pointerBlob.Pointer)
			if err != nil {
				log.Error("Repo[%-v]: Error creating LFS meta object %-v: %v", repo, pointerBlob.Pointer, err)
				return err
			}
		} else {
			if setting.LFS.MaxFileSize > 0 && pointerBlob.Size > setting.LFS.MaxFileSize {
				log.Info("Repo[%-v]: LFS object %-v download denied because of LFS_MAX_FILE_SIZE=%d < size %d", repo, pointerBlob.Pointer, setting.LFS.MaxFileSize, pointerBlob.Size)
				continue
			}

			batch = append(batch, pointerBlob.Pointer)
			if len(batch) >= lfsClient.BatchSize() {
				if err := downloadObjects(batch); err != nil {
					return err
				}
				batch = nil
			}
		}
	}
	if len(batch) > 0 {
		if err := downloadObjects(batch); err != nil {
			return err
		}
	}

	err, has := <-errChan
	if has {
		log.Error("Repo[%-v]: Error enumerating LFS objects for repository: %v", repo, err)
		return err
	}

	return nil
}
