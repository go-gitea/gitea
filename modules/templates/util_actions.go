// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"

	git_model "code.gitea.io/gitea/models/git"
	actions_module "code.gitea.io/gitea/modules/actions"
)

type ActionsUtils struct {
	ctx context.Context
}

func NewActionsUtils(ctx context.Context) *ActionsUtils {
	return &ActionsUtils{ctx: ctx}
}

func (a *ActionsUtils) CommitStatusesToActionsStatuses(statuses []*git_model.CommitStatus) actions_module.CommitActionsStatusMap {
	return actions_module.GetCommitActionsStatusMap(a.ctx, statuses)
}
