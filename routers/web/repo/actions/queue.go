// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	shared_actions "gitea.dev/routers/web/shared/actions"
	"gitea.dev/services/context"
)

// Queue renders this repository's Actions build queue (queued jobs in pickup order plus running jobs)
// inside the Actions tab. Any actions reader may view it; only repo admins get the drag-to-reorder handles.
func Queue(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsActions"] = true
	ctx.Data["PageIsActionsQueue"] = true
	shared_actions.RenderQueue(ctx, shared_actions.QueueScope{
		RepoID:           ctx.Repo.Repository.ID,
		IsRepo:           true,
		CanReorder:       ctx.Repo.Permission.IsAdmin(),
		ShowRunnerColumn: true,
		MoveLink:         ctx.Repo.RepoLink + "/actions/queue/move",
		FullTemplate:     "repo/actions/queue",
	})
}

// QueueMovePost applies a drag-and-drop reorder of this repository's queue. The route is gated to repo admins.
func QueueMovePost(ctx *context.Context) {
	shared_actions.HandleQueueMove(ctx, ctx.Repo.Repository.ID, 0)
}
