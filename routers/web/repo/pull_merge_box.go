// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"html/template"

	"gitea.dev/modules/htmlutil"
	"gitea.dev/services/context"
)

type pullMergeBoxSection struct {
	StatusIcon       string
	StatusIconClass  string
	SectionTitle     template.HTML
	SectionDetails   []template.HTML
	FileNames        []string
	ProgressSegments []statusCheckProgressSegment // drawn as a donut chart instead of the status icon
}

func (prInfo *pullRequestViewInfo) prepareMergeBoxClosedSection(ctx *context.Context) {
	pull := prInfo.issue.PullRequest
	data := prInfo.MergeBoxData
	headTarget := htmlutil.HTMLFormat("<code>%s</code>", prInfo.headTarget)
	title, detail := ctx.Locale.Tr("repo.pulls.closed_with_unmerged_commits"), ctx.Locale.Tr("repo.pulls.is_closed")
	switch {
	case pull.HasMerged:
		title, detail = ctx.Locale.Tr("repo.pulls.merged_success"), ctx.Locale.Tr("repo.pulls.merged_info_desc")
		if data.IsPullBranchDeletable {
			detail = ctx.Locale.Tr("repo.pulls.merged_info_text", headTarget)
		}
	case prInfo.IsPullRequestBroken:
		title, detail = ctx.Locale.Tr("repo.pulls.closed"), ctx.Locale.Tr("repo.pulls.cant_reopen_deleted_branch")
	case data.IsPullBranchDeletable:
		detail = ctx.Locale.Tr("repo.pulls.closed_with_unmerged_commits_desc", headTarget)
	}
	data.MergeSection = &pullMergeBoxSection{SectionTitle: title, SectionDetails: []template.HTML{detail}}
}

func (prInfo *pullRequestViewInfo) prepareMergeBoxSections(ctx *context.Context) {
	pull := prInfo.issue.PullRequest
	data := prInfo.MergeBoxData

	section := &pullMergeBoxSection{StatusIcon: "octicon-alert-fill", StatusIconClass: "tw-bg-grey-light"}
	setFailed := func() { section.StatusIcon, section.StatusIconClass = "octicon-x", "tw-bg-red" }
	setDetail := func(detail template.HTML) { section.SectionDetails = []template.HTML{detail} }
	switch {
	case prInfo.IsPullRequestBroken:
		setFailed()
		section.SectionTitle = ctx.Locale.Tr("repo.pulls.data_broken")
	case pull.IsFilesConflicted():
		section.SectionTitle = ctx.Locale.Tr("repo.pulls.files_conflicted")
		setDetail(ctx.Locale.Tr("repo.pulls.files_conflicted_desc"))
		section.FileNames = pull.ConflictedFiles
		if len(section.FileNames) > 10 {
			section.FileNames = append(section.FileNames[:10:10], "…")
		}
	case pull.IsChecking():
		section.StatusIcon = "octicon-sync"
		section.SectionTitle = ctx.Locale.Tr("repo.pulls.is_checking")
		setDetail(ctx.Locale.Tr("repo.pulls.is_checking_desc"))
	case pull.IsAncestor():
		section.SectionTitle = ctx.Locale.Tr("repo.pulls.is_ancestor")
	case pull.IsUnrelated():
		setFailed()
		section.SectionTitle = ctx.Locale.Tr("repo.pulls.is_unrelated")
	case pull.IsEmpty():
		section.SectionTitle = ctx.Locale.Tr("repo.pulls.is_empty")
		setDetail(ctx.Locale.Tr("repo.pulls.can_auto_merge_desc"))
	case !pull.IsStatusMergeable():
		setFailed()
		section.SectionTitle = ctx.Locale.Tr("repo.pulls.cannot_auto_merge_desc")
		setDetail(ctx.Locale.Tr("repo.pulls.cannot_auto_merge_helper"))
	case data.ShowUpdatePullInfo:
		section.SectionTitle = ctx.Locale.Tr("repo.pulls.outdated_with_base_branch")
		setDetail(ctx.Locale.Tr("repo.pulls.outdated_with_base_branch_desc", htmlutil.HTMLFormat("<code>%s</code>", pull.BaseBranch)))
	default:
		section.StatusIcon, section.StatusIconClass = "octicon-check", "tw-bg-green"
		section.SectionTitle = ctx.Locale.Tr("repo.pulls.no_conflicts")
		setDetail(ctx.Locale.Tr("repo.pulls.can_auto_merge_desc"))
	}
	data.MergeSection = section
	if pull.IsChecking() {
		data.ReviewSection = nil
	}

	if prInfo.workInProgressPrefix != "" {
		data.WorkInProgressSection = &pullMergeBoxSection{
			StatusIcon: "octicon-git-pull-request-draft", StatusIconClass: "tw-bg-grey-light", SectionTitle: ctx.Locale.Tr("repo.pulls.cannot_merge_work_in_progress"),
			SectionDetails: []template.HTML{ctx.Locale.Tr("repo.pulls.work_in_progress_desc")},
		}
	}
	if len(data.mergeBlockers) > 0 && data.WorkInProgressSection == nil {
		data.BlockedSection = &pullMergeBoxSection{
			StatusIcon: "octicon-alert-fill", StatusIconClass: "tw-bg-red", SectionTitle: ctx.Locale.Tr("repo.pulls.merging_is_blocked"),
			SectionDetails: data.mergeBlockers, FileNames: pull.ChangedProtectedFiles,
		}
	}

	isMergeable := !prInfo.IsPullRequestBroken && pull.IsStatusMergeable() && prInfo.workInProgressPrefix == ""
	isBlocked := len(data.mergeBlockers) > 0 || data.hasOverridableBlockers
	checksPassed := data.StatusCheckData == nil || data.StatusCheckData.AllPassed()
	data.IsReady = data.hasPermToMerge && isMergeable && !isBlocked && checksPassed
}
