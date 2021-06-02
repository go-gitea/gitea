// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"context"
	"errors"
	"io"
	"net/url"
	"regexp"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

var stripExitStatus = regexp.MustCompile(`exit status \d+ - `)

// AddPushMirrorRemote registers the push mirror remote.
func AddPushMirrorRemote(m *models.PushMirror, addr string) error {
	addRemoteAndConfig := func(addr, path string) error {
		if _, err := git.NewCommand("remote", "add", "--mirror=push", m.RemoteName, addr).RunInDir(path); err != nil {
			return err
		}
		if _, err := git.NewCommand("config", "--add", "remote."+m.RemoteName+".push", "+refs/heads/*:refs/heads/*").RunInDir(path); err != nil {
			return err
		}
		if _, err := git.NewCommand("config", "--add", "remote."+m.RemoteName+".push", "+refs/tags/*:refs/tags/*").RunInDir(path); err != nil {
			return err
		}
		return nil
	}

	if err := addRemoteAndConfig(addr, m.Repo.RepoPath()); err != nil {
		return err
	}

	if m.Repo.HasWiki() {
		wikiRemoteURL := repository.WikiRemoteURL(addr)
		if len(wikiRemoteURL) > 0 {
			if err := addRemoteAndConfig(wikiRemoteURL, m.Repo.WikiPath()); err != nil {
				return err
			}
		}
	}

	return nil
}

// RemovePushMirrorRemote removes the push mirror remote.
func RemovePushMirrorRemote(m *models.PushMirror) error {
	cmd := git.NewCommand("remote", "rm", m.RemoteName)

	if _, err := cmd.RunInDir(m.Repo.RepoPath()); err != nil {
		return err
	}

	if m.Repo.HasWiki() {
		if _, err := cmd.RunInDir(m.Repo.WikiPath()); err != nil {
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

	m, err := models.GetPushMirrorByID(mirrorID)
	if err != nil {
		log.Error("GetPushMirrorByID [%d]: %v", mirrorID, err)
		return false
	}

	m.LastError = ""

	log.Trace("SyncPushMirror [mirror: %d][repo: %-v]: Running Sync", m.ID, m.Repo)
	err = runPushSync(ctx, m)
	if err != nil {
		log.Error("SyncPushMirror [mirror: %d][repo: %-v]: %v", m.ID, m.Repo, err)
		m.LastError = stripExitStatus.ReplaceAllLiteralString(err.Error(), "")
	}

	m.LastUpdateUnix = timeutil.TimeStampNow()

	if err := models.UpdatePushMirror(m); err != nil {
		log.Error("UpdatePushMirror [%d]: %v", m.ID, err)

		return false
	}

	log.Trace("SyncPushMirror [mirror: %d][repo: %-v]: Finished", m.ID, m.Repo)

	return err == nil
}

func runPushSync(ctx context.Context, m *models.PushMirror) error {
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	performPush := func(path string) error {
		remoteAddr, err := git.GetRemoteAddress(path, m.RemoteName)
		if err != nil {
			log.Error("GetRemoteAddress(%s) Error %v", path, err)
			return errors.New("Unexpected error")
		}

		if setting.LFS.StartServer {
			log.Trace("SyncMirrors [repo: %-v]: syncing LFS objects...", m.Repo)

			gitRepo, err := git.OpenRepository(path)
			if err != nil {
				log.Error("OpenRepository: %v", err)
				return errors.New("Unexpected error")
			}
			defer gitRepo.Close()

			ep := lfs.DetermineEndpoint(remoteAddr.String(), "")
			if err := pushAllLFSObjects(ctx, gitRepo, ep); err != nil {
				return util.NewURLSanitizedError(err, remoteAddr, true)
			}
		}

		log.Trace("Pushing %s mirror[%d] remote %s", path, m.ID, m.RemoteName)

		if err := git.Push(path, git.PushOptions{
			Remote:  m.RemoteName,
			Force:   true,
			Mirror:  true,
			Timeout: timeout,
		}); err != nil {
			log.Error("Error pushing %s mirror[%d] remote %s: %v", path, m.ID, m.RemoteName, err)

			return util.NewURLSanitizedError(err, remoteAddr, true)
		}

		return nil
	}

	err := performPush(m.Repo.RepoPath())
	if err != nil {
		return err
	}

	if m.Repo.HasWiki() {
		wikiPath := m.Repo.WikiPath()
		_, err := git.GetRemoteAddress(wikiPath, m.RemoteName)
		if err == nil {
			err := performPush(wikiPath)
			if err != nil {
				return err
			}
		} else {
			log.Trace("Skipping wiki: No remote configured")
		}
	}

	return nil
}

func pushAllLFSObjects(ctx context.Context, gitRepo *git.Repository, endpoint *url.URL) error {
	client := lfs.NewClient(endpoint)
	contentStore := lfs.NewContentStore()

	pointerChan := make(chan lfs.PointerBlob)
	errChan := make(chan error, 1)
	go lfs.SearchPointerBlobs(ctx, gitRepo, pointerChan, errChan)

	uploadObjects := func(pointers []lfs.Pointer) error {
		err := client.Upload(ctx, pointers, func(p lfs.Pointer, objectError error) (io.ReadCloser, error) {
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
		if len(batch) >= client.BatchSize() {
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
