// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"html/template"
	"net/http"
	"slices"

	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	"gitea.dev/modules/htmlutil"
	"gitea.dev/services/context"
	"gitea.dev/services/mergequeue"
	pull_service "gitea.dev/services/pull"
)

func (prInfo *pullRequestViewInfo) prepareQueuedMergeBox(ctx *context.Context) {
	position, err := pull_model.GetMergeQueuePosition(ctx, prInfo.queueEntry)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	data := prInfo.MergeBoxData
	data.IsQueued = true
	data.CanDequeue = ctx.Doer != nil && (ctx.Doer.ID == prInfo.queueEntry.EnqueuerID || prInfo.issue.IsPoster(ctx.Doer.ID) || data.hasPermToMerge)
	queueLink := htmlutil.HTMLFormat(`<a href="%s">%s</a>`, data.MergeQueueLink, ctx.Tr("repo.pulls.merge_queue.link_text"))
	detail := ctx.Tr("repo.pulls.merge_queue.next_up", queueLink)
	if ahead := position - 1; ahead > 0 {
		detail = ctx.TrN(ahead, "repo.pulls.merge_queue.ahead_1", "repo.pulls.merge_queue.ahead_n", ahead, queueLink)
	}
	data.MergeSection = &pullMergeBoxSection{
		StatusIcon: "octicon-git-merge-queue", StatusIconClass: "tw-bg-yellow",
		SectionTitle: ctx.Tr("repo.pulls.merge_queue.queued_title"), SectionDetails: []template.HTML{detail},
	}
	data.ReloadingInterval = 15000
}

func RemoveFromMergeQueue(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	if err := mergequeue.Dequeue(ctx, ctx.Doer, ctx.Repo.Permission, issue.PullRequest); err != nil && !errors.Is(err, mergequeue.ErrNotQueued) {
		ctx.JSONErrorAuto(err)
		return
	}
	ctx.Flash.Success(ctx.Tr("repo.pulls.merge_queue.removed"))
	ctx.JSONOK()
}

func JumpMergeQueue(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	if err := mergequeue.Jump(ctx, issue.PullRequest); err != nil && !errors.Is(err, mergequeue.ErrNotQueued) {
		ctx.JSONErrorAuto(err)
		return
	}
	ctx.JSONOK()
}

func ClearMergeQueue(ctx *context.Context) {
	if err := mergequeue.Clear(ctx, ctx.Doer, ctx.Repo.Repository, ctx.PathParam("*")); err != nil {
		ctx.JSONErrorAuto(err)
		return
	}
	ctx.JSONOK()
}

func MergeQueue(ctx *context.Context) {
	branch, repo := ctx.PathParam("*"), ctx.Repo.Repository
	rule, err := mergequeue.GetEnabledRule(ctx, repo, branch)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	} else if rule == nil {
		ctx.NotFound()
		return
	}
	queue, err := mergequeue.GetQueue(ctx, repo, branch)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	entries := make([]struct {
		*mergequeue.QueueEntry
		CanRemove  bool
		Status     *git_model.CommitStatus
		Statuses   []*git_model.CommitStatus
		WaitingFor []string
	}, len(queue))
	issues := make(issues_model.IssueList, len(queue))
	for i, entry := range queue {
		issues[i] = entry.PullRequest.Issue
	}
	if err := issues.LoadPosters(ctx); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	canMerge := ctx.Doer != nil && git_model.IsUserMergeWhitelisted(ctx, rule, ctx.Doer.ID, ctx.Repo.Permission)
	required, err := pull_service.EffectiveRequiredContexts(ctx, repo, rule)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	required = pull_service.MergeQueueRequiredContexts(required)
	matchers := make([]func(string) bool, len(required))
	for i, name := range required {
		matchers[i] = createRequiredContextMatcher(name)
	}
	for i, entry := range queue {
		view := &entries[i]
		view.QueueEntry = entry
		view.CanRemove = canMerge || ctx.Doer != nil && (ctx.Doer.ID == entry.EnqueuerID || entry.PullRequest.Issue.IsPoster(ctx.Doer.ID))
		if !entry.IsBuilt() {
			continue
		}
		if view.Statuses, err = pull_service.GetLatestPullCommitStatuses(ctx, repo.ID, entry.GroupHeadSHA); err != nil {
			ctx.ServerErrorAuto(err)
			return
		}
		git_model.CommitStatusesApplyDoerPermission(ctx, ctx.Doer, view.Statuses)
		view.Status = git_model.CalcCommitStatus(view.Statuses)
		if entry.State != pull_model.MergeQueueStateAwaitingChecks {
			continue
		}
		for i, matches := range matchers {
			if !slices.ContainsFunc(view.Statuses, func(status *git_model.CommitStatus) bool { return matches(status.Context) }) {
				view.WaitingFor = append(view.WaitingFor, required[i])
			}
		}
	}
	ctx.Data["Title"] = ctx.Tr("repo.pulls.merge_queue.title")
	ctx.Data["PageIsPullList"] = true
	ctx.Data["MergeQueueBranch"] = branch
	ctx.Data["MergeQueueEntries"] = entries
	ctx.HTML(http.StatusOK, "repo/pulls/merge_queue")
}
