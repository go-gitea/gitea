// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/proxy"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	repo_service "code.gitea.io/gitea/services/repository"
)

var stripExitStatus = regexp.MustCompile(`exit status \d+ - `)

// AddPushMirrorRemote registers the push mirror remote.
func AddPushMirrorRemote(ctx context.Context, m *repo_model.PushMirror, addr string) error {
	addRemoteAndConfig := func(storageRepo gitrepo.Repository, addr string) error {
		if err := gitrepo.GitRemoteAdd(ctx, storageRepo, m.RemoteName, addr, gitrepo.RemoteOptionMirrorPush); err != nil {
			return err
		}
		if err := gitrepo.GitConfigAdd(ctx, storageRepo, "remote."+m.RemoteName+".push", "+refs/heads/*:refs/heads/*"); err != nil {
			return err
		}
		return gitrepo.GitConfigAdd(ctx, storageRepo, "remote."+m.RemoteName+".push", "+refs/tags/*:refs/tags/*")
	}

	if err := addRemoteAndConfig(m.Repo, addr); err != nil {
		return err
	}

	if repo_service.HasWiki(ctx, m.Repo) {
		wikiRemoteURL := repository.WikiRemoteURL(ctx, addr)
		if len(wikiRemoteURL) > 0 {
			if err := addRemoteAndConfig(m.Repo.WikiStorageRepo(), wikiRemoteURL); err != nil {
				return err
			}
		}
	}

	return nil
}

// RemovePushMirrorRemote removes the push mirror remote.
func RemovePushMirrorRemote(ctx context.Context, m *repo_model.PushMirror) error {
	_ = m.GetRepository(ctx)
	if err := gitrepo.GitRemoteRemove(ctx, m.Repo, m.RemoteName); err != nil {
		return err
	}

	if repo_service.HasWiki(ctx, m.Repo) {
		if err := gitrepo.GitRemoteRemove(ctx, m.Repo.WikiStorageRepo(), m.RemoteName); err != nil {
			// The wiki remote may not exist
			log.Warn("Wiki Remote[%d] could not be removed: %v", m.ID, err)
		}
	}

	return nil
}

// SyncPushMirror starts the sync of the push mirror and schedules the next run.
func SyncPushMirror(ctx context.Context, mirrorID int64) bool {
	log.Trace("SyncPushMirror [mirror: %d]", mirrorID)
	defer func() {
		err := recover()
		if err == nil {
			return
		}
		// There was a panic whilst syncPushMirror...
		log.Error("PANIC whilst syncPushMirror[%d] Panic: %v\nStacktrace: %s", mirrorID, err, log.Stack(2))
	}()

	// TODO: Handle "!exist" better
	m, exist, err := db.GetByID[repo_model.PushMirror](ctx, mirrorID)
	if err != nil || !exist {
		log.Error("GetPushMirrorByID [%d]: %v", mirrorID, err)
		return false
	}

	_ = m.GetRepository(ctx)

	m.LastError = ""

	ctx, _, finished := process.GetManager().AddContext(ctx, fmt.Sprintf("Syncing PushMirror %s/%s to %s", m.Repo.OwnerName, m.Repo.Name, m.RemoteName))
	defer finished()

	log.Trace("SyncPushMirror [mirror: %d][repo: %-v]: Running Sync", m.ID, m.Repo)
	err = runPushSync(ctx, m)
	if err != nil {
		log.Error("SyncPushMirror [mirror: %d][repo: %-v]: %v", m.ID, m.Repo, err)
		m.LastError = stripExitStatus.ReplaceAllLiteralString(err.Error(), "")
	}

	m.LastUpdateUnix = timeutil.TimeStampNow()

	if err := repo_model.UpdatePushMirror(ctx, m); err != nil {
		log.Error("UpdatePushMirror [%d]: %v", m.ID, err)

		return false
	}

	log.Trace("SyncPushMirror [mirror: %d][repo: %-v]: Finished", m.ID, m.Repo)

	return err == nil
}

func runPushSync(ctx context.Context, m *repo_model.PushMirror) error {
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	performPush := func(repo *repo_model.Repository, isWiki bool) error {
		var storageRepo gitrepo.Repository = repo
		path := repo.RepoPath()
		if isWiki {
			storageRepo = repo.WikiStorageRepo()
			path = repo.WikiPath()
		}
		remoteURL, err := gitrepo.GitRemoteGetURL(ctx, storageRepo, m.RemoteName)
		if err != nil {
			log.Error("GetRemoteURL(%s) Error %v", path, err)
			return errors.New("Unexpected error")
		}

		if setting.LFS.StartServer {
			log.Trace("SyncMirrors [repo: %-v]: syncing LFS objects...", m.Repo)

			gitRepo, err := gitrepo.OpenRepository(ctx, storageRepo)
			if err != nil {
				log.Error("OpenRepository: %v", err)
				return errors.New("Unexpected error")
			}
			defer gitRepo.Close()

			endpoint := lfs.DetermineEndpoint(remoteURL.String(), "")
			lfsClient := lfs.NewClient(endpoint, nil)
			if err := pushAllLFSObjects(ctx, gitRepo, lfsClient); err != nil {
				return util.SanitizeErrorCredentialURLs(err)
			}
		}

		log.Trace("Pushing %s mirror[%d] remote %s", path, m.ID, m.RemoteName)

		envs := proxy.EnvWithProxy(remoteURL.URL)
		if err := git.Push(ctx, path, git.PushOptions{
			Remote:  m.RemoteName,
			Force:   true,
			Mirror:  true,
			Timeout: timeout,
			Env:     envs,
		}); err != nil {
			log.Error("Error pushing %s mirror[%d] remote %s: %v", path, m.ID, m.RemoteName, err)

			return util.SanitizeErrorCredentialURLs(err)
		}

		return nil
	}

	err := performPush(m.Repo, false)
	if err != nil {
		return err
	}

	if repo_service.HasWiki(ctx, m.Repo) {
		if _, err := gitrepo.GitRemoteGetURL(ctx, m.Repo.WikiStorageRepo(), m.RemoteName); err == nil {
			err := performPush(m.Repo, true)
			if err != nil {
				return err
			}
		} else if !errors.Is(err, util.ErrNotExist) {
			log.Error("GetRemote of wiki failed: %v", err)
		}
	}

	return nil
}

func pushAllLFSObjects(ctx context.Context, gitRepo *git.Repository, lfsClient lfs.Client) error {
	contentStore := lfs.NewContentStore()

	pointerChan := make(chan lfs.PointerBlob)
	errChan := make(chan error, 1)
	go lfs.SearchPointerBlobs(ctx, gitRepo, pointerChan, errChan)

	uploadObjects := func(pointers []lfs.Pointer) error {
		err := lfsClient.Upload(ctx, pointers, func(p lfs.Pointer, objectError error) (io.ReadCloser, error) {
			if objectError != nil {
				return nil, objectError
			}

			content, err := contentStore.Get(p)
			if err != nil {
				log.Error("Error reading LFS object %v: %v", p, err)
			}
			return content, err
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
		exists, err := contentStore.Exists(pointerBlob.Pointer)
		if err != nil {
			log.Error("Error checking if LFS object %v exists: %v", pointerBlob.Pointer, err)
			return err
		}
		if !exists {
			log.Trace("Skipping missing LFS object %v", pointerBlob.Pointer)
			continue
		}

		batch = append(batch, pointerBlob.Pointer)
		if len(batch) >= lfsClient.BatchSize() {
			if err := uploadObjects(batch); err != nil {
				return err
			}
			batch = nil
		}
	}
	if len(batch) > 0 {
		if err := uploadObjects(batch); err != nil {
			return err
		}
	}

	err, has := <-errChan
	if has {
		log.Error("Error enumerating LFS objects for repository: %v", err)
		return err
	}

	return nil
}

func syncPushMirrorWithSyncOnCommit(ctx context.Context, repoID int64) {
	pushMirrors, err := repo_model.GetPushMirrorsSyncedOnCommit(ctx, repoID)
	if err != nil {
		log.Error("repo_model.GetPushMirrorsSyncedOnCommit failed: %v", err)
		return
	}

	for _, mirror := range pushMirrors {
		AddPushMirrorToQueue(mirror.ID)
	}
}
