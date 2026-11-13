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
	HasMoreFiles     bool
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
	switch {
	case prInfo.IsPullRequestBroken:
		section.StatusIcon, section.StatusIconClass, section.SectionTitle = "octicon-x", "tw-bg-red", ctx.Locale.Tr("repo.pulls.data_broken")
	case pull.IsFilesConflicted():
		section.SectionTitle, section.SectionDetails = ctx.Locale.Tr("repo.pulls.files_conflicted"), []template.HTML{ctx.Locale.Tr("repo.pulls.files_conflicted_desc")}
		section.FileNames = pull.ConflictedFiles[:min(len(pull.ConflictedFiles), 10)]
		section.HasMoreFiles = len(pull.ConflictedFiles) > len(section.FileNames)
	case pull.IsChecking():
		section.StatusIcon, section.SectionTitle, section.SectionDetails = "octicon-sync", ctx.Locale.Tr("repo.pulls.is_checking"), []template.HTML{ctx.Locale.Tr("repo.pulls.is_checking_desc")}
	case pull.IsAncestor():
		section.SectionTitle = ctx.Locale.Tr("repo.pulls.is_ancestor")
	case pull.IsUnrelated():
		section.StatusIcon, section.StatusIconClass, section.SectionTitle = "octicon-x", "tw-bg-red", ctx.Locale.Tr("repo.pulls.is_unrelated")
	case pull.IsEmpty():
		section.SectionTitle, section.SectionDetails = ctx.Locale.Tr("repo.pulls.is_empty"), []template.HTML{ctx.Locale.Tr("repo.pulls.can_auto_merge_desc")}
	case !pull.IsStatusMergeable():
		section.StatusIcon, section.StatusIconClass, section.SectionTitle = "octicon-x", "tw-bg-red", ctx.Locale.Tr("repo.pulls.merge_unavailable")
	case data.ShowUpdatePullInfo:
		section.SectionTitle = ctx.Locale.Tr("repo.pulls.outdated_with_base_branch")
		section.SectionDetails = []template.HTML{ctx.Locale.Tr("repo.pulls.outdated_with_base_branch_desc", htmlutil.HTMLFormat("<code>%s</code>", pull.BaseBranch))}
	default:
		section.StatusIcon, section.StatusIconClass, section.SectionTitle = "octicon-check", "tw-bg-green", ctx.Locale.Tr("repo.pulls.no_conflicts")
		section.SectionDetails = []template.HTML{ctx.Locale.Tr("repo.pulls.can_auto_merge_desc")}
	}
	data.MergeSection = section

	if prInfo.workInProgressPrefix != "" {
		data.WorkInProgressSection = &pullMergeBoxSection{
			StatusIcon: "octicon-git-pull-request-draft", StatusIconClass: "tw-bg-grey-light", SectionTitle: ctx.Locale.Tr("repo.pulls.cannot_merge_work_in_progress"),
			SectionDetails: []template.HTML{ctx.Locale.Tr("repo.pulls.work_in_progress_desc")},
		}
	}
	if len(data.mergeBlockers) > 0 {
		data.BlockedSection = &pullMergeBoxSection{
			StatusIcon: "octicon-alert-fill", StatusIconClass: "tw-bg-red", SectionTitle: ctx.Locale.Tr("repo.pulls.merging_is_blocked"),
			SectionDetails: data.mergeBlockers, FileNames: pull.ChangedProtectedFiles,
		}
	}
}
