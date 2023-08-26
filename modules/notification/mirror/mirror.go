// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/repository"
	mirror_service "code.gitea.io/gitea/services/mirror"
)

type mirrorNotifier struct {
	base.NullNotifier
}

var _ base.Notifier = &mirrorNotifier{}

// NewNotifier create a new mirrorNotifier notifier
func NewNotifier() base.Notifier {
	return &mirrorNotifier{}
}

func (m *mirrorNotifier) NotifyPushCommits(ctx context.Context, _ *user_model.User, repo *repo_model.Repository, _ *repository.PushUpdateOptions, _ *repository.PushCommits) {
	mirror_service.SyncPushMirrorWithSyncOnCommit(ctx, repo.ID)
}

func (m *mirrorNotifier) NotifySyncPushCommits(ctx context.Context, _ *user_model.User, repo *repo_model.Repository, _ *repository.PushUpdateOptions, _ *repository.PushCommits) {
	mirror_service.SyncPushMirrorWithSyncOnCommit(ctx, repo.ID)
}
