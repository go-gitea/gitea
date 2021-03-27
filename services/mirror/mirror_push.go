// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"fmt"
	"strconv"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func syncPushMirror(mirrorID string) {
	log.Trace("SyncPushMirror [mirror_id: %v]", mirrorID)
	defer func() {
		err := recover()
		if err == nil {
			return
		}
		// There was a panic whilst syncPushMirror...
		log.Error("PANIC whilst syncPushMirror[%s] Panic: %v\nStacktrace: %s", mirrorID, err, log.Stack(2))
	}()

	id, _ := strconv.ParseInt(mirrorID, 10, 64)
	m, err := models.GetPushMirrorByID(id)
	if err != nil {
		log.Error("GetPushMirrorByID [%d]: %v", id, err)
		return
	}

	log.Trace("SyncPushMirror [repo: %-v]: Running Sync", m.Repo)
	err = runPushSync(m)
	if err != nil {
		log.Error("SyncPushMirror [%d]: %v", id, err)
		return
	}

	log.Trace("SyncPushMirror [repo: %-v]: Scheduling next update", m.Repo)
	m.ScheduleNextUpdate()
	if err = models.UpdatePushMirror(m); err != nil {
		log.Error("UpdatePushMirror [%d]: %v", id, err)
		return
	}

	log.Trace("SyncPushMirror [repo: %-v]: Successfully updated", m.Repo)
}

// runPushSync returns true if sync finished without error.
func runPushSync(m *models.PushMirror) error {
	repoPath := m.Repo.RepoPath()
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	log.Trace("SyncPushMirror [repo: %-v]: running git push...", m.Repo)

	performPush := func(path string) error {
		if err := git.Push(path, git.PushOptions{
			Remote:  m.RemoteName,
			Force:   true,
			Mirror:  true,
			Timeout: timeout,
		}); err != nil {
			if git.IsErrPushOutOfDate(err) || git.IsErrPushRejected(err) {
				return err
			}
			return fmt.Errorf("Error pushing remote %s to %s: %v", m.RemoteName, path, err)
		}
		return nil
	}

	err := performPush(repoPath)
	if err != nil {
		return err
	}

	// TODO LFS

	if m.Repo.HasWiki() {
		err := performPush(m.Repo.WikiPath());
		if err != nil {
			return nil
		}
	}

	return nil
}
