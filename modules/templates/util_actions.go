// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/modules/actions/statusinfo"
)

// ActionsUtils groups template helpers for Gitea Actions data. Methods may
// issue DB queries; unlike MiscUtils/RenderUtils these are not pure.
type ActionsUtils struct {
	ctx context.Context
}

func NewActionsUtils(ctx context.Context) *ActionsUtils {
	return &ActionsUtils{ctx: ctx}
}

// CommitStatusInfo resolves the live ActionRunJob.Status for every
// Gitea-Actions-backed CommitStatus row so repo/pulls/status.tmpl can render
// the matching live icon (the stored State collapses Waiting/Running/Blocked
// into Pending).
func (a *ActionsUtils) CommitStatusInfo(statuses []*git_model.CommitStatus) statusinfo.ActionInfo {
	return statusinfo.GetActionInfo(a.ctx, statuses)
}
