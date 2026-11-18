// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	pull_model "gitea.dev/models/pull"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	"gitea.dev/services/mergequeue"
)

const tplMergeQueue templates.TplName = "repo/pulls/merge_queue"

type pullMergeQueueData struct {
	BaseBranch string
	QueueLink  string
	Entry      *pull_model.MergeQueueEntry
	Position   int
	CanDequeue bool
}

// prepareMergeBoxMergeQueue shows the queue position of a queued pull request, which can't be merged in another way meanwhile
func (prInfo *pullRequestViewInfo) prepareMergeBoxMergeQueue(ctx *context.Context) {
	pull := prInfo.issue.PullRequest
	pb := prInfo.ProtectedBranchRule
	if pb == nil || !pb.RequireMergeQueue || pull.HasMerged || prInfo.issue.IsClosed {
		return
	}
	entry, position, err := mergequeue.GetEntry(ctx, pull)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	data := prInfo.MergeBoxData
	data.MergeQueue = &pullMergeQueueData{
		BaseBranch: pull.BaseBranch,
		QueueLink:  ctx.Repo.RepoLink + "/queue/" + util.PathEscapeSegments(pull.BaseBranch),
		Entry:      entry,
		Position:   position,
		// same rule as mergequeue.Dequeue, reusing the merge permission computed for the merge box
		CanDequeue: entry != nil && ctx.Doer != nil && (ctx.Doer.ID == entry.EnqueuerID || data.hasPermToMerge),
	}
	if entry != nil {
		// queue progress follows CI runs, so a slower refresh is enough
		data.ReloadingInterval = util.IfZero(data.ReloadingInterval, 15000)
	}
}

func RemoveFromMergeQueue(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	if err := mergequeue.Dequeue(ctx, ctx.Doer, ctx.Repo.Permission, issue.PullRequest); err != nil && !errors.Is(err, mergequeue.ErrNotQueued) {
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Flash.Success(ctx.Tr("repo.pulls.merge_queue.removed"))
	ctx.JSONRedirect(issue.Link())
}

func MergeQueue(ctx *context.Context) {
	branch := ctx.PathParam("*")
	queue, err := mergequeue.GetQueue(ctx, ctx.Repo.Repository.ID, branch)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	for _, e := range queue {
		e.PullRequest.Issue.Repo = ctx.Repo.Repository
	}
	ctx.Data["Title"] = ctx.Tr("repo.pulls.merge_queue.title", branch)
	ctx.Data["PageIsPullList"] = true
	ctx.Data["MergeQueueEntries"] = queue
	ctx.HTML(http.StatusOK, tplMergeQueue)
}
