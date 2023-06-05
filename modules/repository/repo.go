// Copyright 2019 The Gitea Authors. All rights reserved.
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
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

/*
GitHub, GitLab, Gogs: *.wiki.git
BitBucket: *.git/wiki
*/
var commonWikiURLSuffixes = []string{".wiki.git", ".git/wiki"}

// WikiRemoteURL returns accessible repository URL for wiki if exists.
// Otherwise, it returns an empty string.
func WikiRemoteURL(ctx context.Context, remote string) string {
	remote = strings.TrimSuffix(remote, ".git")
	for _, suffix := range commonWikiURLSuffixes {
		wikiURL := remote + suffix
		if git.IsRepoURLAccessible(ctx, wikiURL) {
			return wikiURL
		}
	}
	return ""
}

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

		if !opts.Releases {
			// note: this will greatly improve release (tag) sync
			// for pull-mirrors with many tags
			repo.IsMirror = opts.Mirror
			if err = SyncReleasesWithTags(repo, gitRepo); err != nil {
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

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	if opts.Mirror {
		mirrorModel := repo_model.Mirror{
			RepoID:         repo.ID,
			Interval:       setting.Mirror.DefaultInterval,
			EnablePrune:    true,
			NextUpdateUnix: timeutil.TimeStampNow().AddDuration(setting.Mirror.DefaultInterval),
			LFS:            opts.LFS,
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
	if err := createDelegateHooks(repoPath); err != nil {
		return repo, fmt.Errorf("createDelegateHooks: %w", err)
	}
	if repo.HasWiki() {
		if err := createDelegateHooks(repo.WikiPath()); err != nil {
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

// SyncReleasesWithTags synchronizes release table with repository tags
func SyncReleasesWithTags(repo *repo_model.Repository, gitRepo *git.Repository) error {
	log.Debug("SyncReleasesWithTags: in Repo[%d:%s/%s]", repo.ID, repo.OwnerName, repo.Name)

	// optimized procedure for pull-mirrors which saves a lot of time (in
	// particular for repos with many tags).
	if repo.IsMirror {
		return pullMirrorReleaseSync(repo, gitRepo)
	}

	existingRelTags := make(container.Set[string])
	opts := repo_model.FindReleasesOptions{
		IncludeDrafts: true,
		IncludeTags:   true,
		ListOptions:   db.ListOptions{PageSize: 50},
	}
	for page := 1; ; page++ {
		opts.Page = page
		rels, err := repo_model.GetReleasesByRepoID(gitRepo.Ctx, repo.ID, opts)
		if err != nil {
			return fmt.Errorf("unable to GetReleasesByRepoID in Repo[%d:%s/%s]: %w", repo.ID, repo.OwnerName, repo.Name, err)
		}
		if len(rels) == 0 {
			break
		}
		for _, rel := range rels {
			if rel.IsDraft {
				continue
			}
			commitID, err := gitRepo.GetTagCommitID(rel.TagName)
			if err != nil && !git.IsErrNotExist(err) {
				return fmt.Errorf("unable to GetTagCommitID for %q in Repo[%d:%s/%s]: %w", rel.TagName, repo.ID, repo.OwnerName, repo.Name, err)
			}
			if git.IsErrNotExist(err) || commitID != rel.Sha1 {
				if err := repo_model.PushUpdateDeleteTag(repo, rel.TagName); err != nil {
					return fmt.Errorf("unable to PushUpdateDeleteTag: %q in Repo[%d:%s/%s]: %w", rel.TagName, repo.ID, repo.OwnerName, repo.Name, err)
				}
			} else {
				existingRelTags.Add(strings.ToLower(rel.TagName))
			}
		}
	}

	_, err := gitRepo.WalkReferences(git.ObjectTag, 0, 0, func(sha1, refname string) error {
		tagName := strings.TrimPrefix(refname, git.TagPrefix)
		if existingRelTags.Contains(strings.ToLower(tagName)) {
			return nil
		}

		if err := PushUpdateAddTag(db.DefaultContext, repo, gitRepo, tagName, sha1, refname); err != nil {
			return fmt.Errorf("unable to PushUpdateAddTag: %q to Repo[%d:%s/%s]: %w", tagName, repo.ID, repo.OwnerName, repo.Name, err)
		}

		return nil
	})
	return err
}

// PushUpdateAddTag must be called for any push actions to add tag
func PushUpdateAddTag(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, tagName, sha1, refname string) error {
	tag, err := gitRepo.GetTagWithID(sha1, tagName)
	if err != nil {
		return fmt.Errorf("unable to GetTag: %w", err)
	}
	commit, err := tag.Commit(gitRepo)
	if err != nil {
		return fmt.Errorf("unable to get tag Commit: %w", err)
	}

	sig := tag.Tagger
	if sig == nil {
		sig = commit.Author
	}
	if sig == nil {
		sig = commit.Committer
	}

	var author *user_model.User
	createdAt := time.Unix(1, 0)

	if sig != nil {
		author, err = user_model.GetUserByEmail(ctx, sig.Email)
		if err != nil && !user_model.IsErrUserNotExist(err) {
			return fmt.Errorf("unable to GetUserByEmail for %q: %w", sig.Email, err)
		}
		createdAt = sig.When
	}

	commitsCount, err := commit.CommitsCount()
	if err != nil {
		return fmt.Errorf("unable to get CommitsCount: %w", err)
	}

	rel := repo_model.Release{
		RepoID:       repo.ID,
		TagName:      tagName,
		LowerTagName: strings.ToLower(tagName),
		Sha1:         commit.ID.String(),
		NumCommits:   commitsCount,
		CreatedUnix:  timeutil.TimeStamp(createdAt.Unix()),
		IsTag:        true,
	}
	if author != nil {
		rel.PublisherID = author.ID
	}

	return repo_model.SaveOrUpdateTag(repo, &rel)
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

			_, err := git_model.NewLFSMetaObject(ctx, &git_model.LFSMetaObject{Pointer: p, RepositoryID: repo.ID})
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
			_, err := git_model.NewLFSMetaObject(ctx, &git_model.LFSMetaObject{Pointer: pointerBlob.Pointer, RepositoryID: repo.ID})
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

// pullMirrorReleaseSync is a pull-mirror specific tag<->release table
// synchronization which overwrites all Releases from the repository tags. This
// can be relied on since a pull-mirror is always identical to its
// upstream. Hence, after each sync we want the pull-mirror release set to be
// identical to the upstream tag set. This is much more efficient for
// repositories like https://github.com/vim/vim (with over 13000 tags).
func pullMirrorReleaseSync(repo *repo_model.Repository, gitRepo *git.Repository) error {
	log.Trace("pullMirrorReleaseSync: rebuilding releases for pull-mirror Repo[%d:%s/%s]", repo.ID, repo.OwnerName, repo.Name)
	tags, numTags, err := gitRepo.GetTagInfos(0, 0)
	if err != nil {
		return fmt.Errorf("unable to GetTagInfos in pull-mirror Repo[%d:%s/%s]: %w", repo.ID, repo.OwnerName, repo.Name, err)
	}
	err = db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		//
		// clear out existing releases
		//
		if _, err := db.DeleteByBean(ctx, &repo_model.Release{RepoID: repo.ID}); err != nil {
			return fmt.Errorf("unable to clear releases for pull-mirror Repo[%d:%s/%s]: %w", repo.ID, repo.OwnerName, repo.Name, err)
		}
		//
		// make release set identical to upstream tags
		//
		for _, tag := range tags {
			release := repo_model.Release{
				RepoID:       repo.ID,
				TagName:      tag.Name,
				LowerTagName: strings.ToLower(tag.Name),
				Sha1:         tag.Object.String(),
				// NOTE: ignored, since NumCommits are unused
				// for pull-mirrors (only relevant when
				// displaying releases, IsTag: false)
				NumCommits:  -1,
				CreatedUnix: timeutil.TimeStamp(tag.Tagger.When.Unix()),
				IsTag:       true,
			}
			if err := db.Insert(ctx, release); err != nil {
				return fmt.Errorf("unable insert tag %s for pull-mirror Repo[%d:%s/%s]: %w", tag.Name, repo.ID, repo.OwnerName, repo.Name, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to rebuild release table for pull-mirror Repo[%d:%s/%s]: %w", repo.ID, repo.OwnerName, repo.Name, err)
	}

	log.Trace("pullMirrorReleaseSync: done rebuilding %d releases", numTags)
	return nil
}
