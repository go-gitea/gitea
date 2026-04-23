// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"

	git_model "code.gitea.io/gitea/models/git"
	actions_module "code.gitea.io/gitea/modules/actions"
)

// ActionsUtils groups template helpers for Gitea Actions data. Methods may
// issue DB queries.
type ActionsUtils struct {
	ctx context.Context
}

func NewActionsUtils(ctx context.Context) *ActionsUtils {
	return &ActionsUtils{ctx: ctx}
}

func (a *ActionsUtils) CommitStatusInfo(statuses []*git_model.CommitStatus) actions_module.CommitStatusInfo {
	return actions_module.GetCommitStatusInfo(a.ctx, statuses)
}
