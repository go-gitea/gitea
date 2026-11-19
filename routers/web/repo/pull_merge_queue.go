// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"slices"

	git_model "gitea.dev/models/git"
	pull_model "gitea.dev/models/pull"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	"gitea.dev/services/mergequeue"
	pull_service "gitea.dev/services/pull"
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
	queue, err := mergequeue.GetQueue(ctx, ctx.Repo.Repository, branch)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	waitingFor, err := mergeQueueWaitingFor(ctx, branch, queue)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Data["MergeQueueWaitingFor"] = waitingFor
	ctx.Data["Title"] = ctx.Tr("repo.pulls.merge_queue.title", branch)
	ctx.Data["PageIsPullList"] = true
	ctx.Data["MergeQueueEntries"] = queue
	ctx.HTML(http.StatusOK, tplMergeQueue)
}

// mergeQueueWaitingFor returns the required contexts not reported yet for each queue commit, a queue stalls when nothing reports them
func mergeQueueWaitingFor(ctx *context.Context, branch string, queue []*mergequeue.QueueEntry) (map[int64][]string, error) {
	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, ctx.Repo.Repository.ID, branch)
	if err != nil {
		return nil, err
	}
	required, err := pull_service.EffectiveRequiredContexts(ctx, ctx.Repo.Repository, pb)
	if err != nil {
		return nil, err
	}
	required = pull_service.MergeQueueRequiredContexts(required)
	waitingFor := map[int64][]string{}
	for _, e := range queue {
		if e.State != pull_model.MergeQueueStateAwaitingChecks {
			continue
		}
		statuses, err := pull_service.GetLatestPullCommitStatuses(ctx, ctx.Repo.Repository.ID, e.GroupHeadSHA)
		if err != nil {
			return nil, err
		}
		for _, c := range required {
			matches := createRequiredContextMatcher(c)
			if !slices.ContainsFunc(statuses, func(status *git_model.CommitStatus) bool { return matches(status.Context) }) {
				waitingFor[e.ID] = append(waitingFor[e.ID], c)
			}
		}
	}
	return waitingFor, nil
}
