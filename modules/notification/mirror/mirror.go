// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/repository"
	pushmirror_service "code.gitea.io/gitea/services/pushmirror"
)

type mirrorNotifier struct {
	base.NullNotifier
}

var _ base.Notifier = &mirrorNotifier{}

// NewNotifier create a new mirrorNotifier notifier
func NewNotifier() base.Notifier {
	return &mirrorNotifier{}
}

func (m *mirrorNotifier) NotifyPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("mirrorNotifier.NotifyPushCommits User: %s[%d] in %s[%d]", pusher.Name, pusher.ID, repo.FullName(), repo.ID))
	defer finished()

	syncOnPush := true
	pushMirrors, err := repo_model.GetPushMirrorsByRepoIDWithSyncOnPush(repo.ID, syncOnPush)
	if err != nil {
		log.Error("repo_model.GetPushMirrorsByRepoIDWithSyncOnPush failed: %v", err)
		return
	}

	for _, mirror := range pushMirrors {
		// TODO: push mirror likely will benefit from using a queue
		pushmirror_service.SyncPushMirror(ctx, mirror.ID)
	}
}
