// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	git_model "gitea.dev/models/git"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/reqctx"
)

type ActionsUtils struct {
	ctx reqctx.RequestContext
}

func NewActionsUtils(ctx reqctx.RequestContext) *ActionsUtils {
	return &ActionsUtils{ctx: ctx}
}

func (a *ActionsUtils) CommitStatusesToActionsStatuses(statuses []*git_model.CommitStatus) actions_module.CommitActionsStatusMap {
	return actions_module.GetCommitActionsStatusMapCached(a.ctx, a.ctx.GetData(), statuses)
}
