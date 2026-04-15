// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	notify_service "code.gitea.io/gitea/services/notify"
)

func init() {
	notify_service.RegisterNotifier(&contributorStatsNotifier{})
}

type contributorStatsNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &contributorStatsNotifier{}

func (c *contributorStatsNotifier) PushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repo_module.PushUpdateOptions, commits *repo_module.PushCommits) {
	c.enqueueUpdate(repo, opts)
}

func (c *contributorStatsNotifier) SyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repo_module.PushUpdateOptions, commits *repo_module.PushCommits) {
	c.enqueueUpdate(repo, opts)
}

func (c *contributorStatsNotifier) ChangeDefaultBranch(ctx context.Context, repo *repo_model.Repository) {
	if err := RequestContributorStatsRebuild(ctx, repo.ID); err != nil && !errors.Is(err, ErrAwaitGeneration) {
		log.Error("RequestContributorStatsRebuild %s/%s failed: %v", repo.OwnerName, repo.Name, err)
	}
}

func (c *contributorStatsNotifier) enqueueUpdate(repo *repo_model.Repository, opts *repo_module.PushUpdateOptions) {
	if opts.RefFullName.IsBranch() && opts.RefFullName.ShortName() == repo.DefaultBranch && !opts.IsDelRef() {
		if err := enqueueContributorStatsUpdate(repo.ID, opts.OldCommitID, opts.NewCommitID); err != nil {
			log.Error("enqueueContributorStatsUpdate %s/%s failed: %v", repo.OwnerName, repo.Name, err)
		}
	}
}
