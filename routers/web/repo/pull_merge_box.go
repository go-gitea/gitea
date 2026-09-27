// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"html/template"

	"gitea.dev/modules/htmlutil"
	"gitea.dev/services/context"
)

type pullMergeBoxSection struct {
	Icon      string
	IconClass string
	Title     template.HTML
	Details   []template.HTML
	Files     []string
	Ring      []statusCheckRingSegment
}

const (
	sectionColorSuccess = "green"
	sectionColorDanger  = "red"
	sectionColorNeutral = "grey"
)

func (prInfo *pullRequestViewInfo) prepareMergeBoxClosedSection(ctx *context.Context) {
	pull := prInfo.issue.PullRequest
	data := prInfo.MergeBoxData
	headTarget := htmlutil.HTMLFormat("<code>%s</code>", prInfo.headTarget)
	title, detail := ctx.Locale.Tr("repo.pulls.closed_with_unmerged_commits"), ctx.Locale.Tr("repo.pulls.is_closed")
	switch {
	case pull.HasMerged:
		title, detail = ctx.Locale.Tr("repo.pulls.merged_success"), ctx.Locale.Tr("repo.pulls.merged_info_text", headTarget)
	case prInfo.IsPullRequestBroken:
		title, detail = ctx.Locale.Tr("repo.pulls.closed"), ctx.Locale.Tr("repo.pulls.cant_reopen_deleted_branch")
	case data.IsPullBranchDeletable:
		detail = ctx.Locale.Tr("repo.pulls.closed_with_unmerged_commits_desc", headTarget)
	}
	data.ClosedSection = &pullMergeBoxSection{Title: title, Details: []template.HTML{detail}}
}

func (prInfo *pullRequestViewInfo) prepareMergeBoxInfoItems(ctx *context.Context) {
	pull := prInfo.issue.PullRequest
	data := prInfo.MergeBoxData

	section := &pullMergeBoxSection{Icon: "octicon-alert-fill", IconClass: sectionColorNeutral}
	switch {
	case prInfo.IsPullRequestBroken:
		section.Icon, section.IconClass, section.Title = "octicon-x", sectionColorDanger, ctx.Locale.Tr("repo.pulls.data_broken")
	case pull.IsFilesConflicted():
		section.Title, section.Details = ctx.Locale.Tr("repo.pulls.files_conflicted"), []template.HTML{ctx.Locale.Tr("repo.pulls.files_conflicted_desc")}
		section.Files = pull.ConflictedFiles
		if len(section.Files) > 10 {
			section.Files = append(section.Files[:10:10], "…")
		}
	case pull.IsChecking():
		section.Icon, section.Title, section.Details = "octicon-sync", ctx.Locale.Tr("repo.pulls.is_checking"), []template.HTML{ctx.Locale.Tr("repo.pulls.is_checking_desc")}
	case pull.IsAncestor():
		section.Title = ctx.Locale.Tr("repo.pulls.is_ancestor")
	case pull.IsEmpty():
		section.Title, section.Details = ctx.Locale.Tr("repo.pulls.is_empty"), []template.HTML{ctx.Locale.Tr("repo.pulls.can_auto_merge_desc")}
	case !pull.IsStatusMergeable():
		section.Icon, section.IconClass, section.Title = "octicon-x", sectionColorDanger, ctx.Locale.Tr("repo.pulls.cannot_auto_merge_desc")
		section.Details = []template.HTML{ctx.Locale.Tr("repo.pulls.cannot_auto_merge_helper")}
	case data.ShowUpdatePullInfo:
		section.Title = ctx.Locale.Tr("repo.pulls.outdated_with_base_branch")
		section.Details = []template.HTML{ctx.Locale.Tr("repo.pulls.outdated_with_base_branch_desc", htmlutil.HTMLFormat("<code>%s</code>", pull.BaseBranch))}
	default:
		section.Icon, section.IconClass, section.Title = "octicon-check", sectionColorSuccess, ctx.Locale.Tr("repo.pulls.no_conflicts")
		section.Details = []template.HTML{ctx.Locale.Tr("repo.pulls.can_auto_merge_desc")}
	}
	data.MergeSection = section

	if prInfo.workInProgressPrefix != "" {
		data.WorkInProgressSection = &pullMergeBoxSection{
			Icon: "octicon-git-pull-request-draft", IconClass: sectionColorNeutral, Title: ctx.Locale.Tr("repo.pulls.cannot_merge_work_in_progress"),
			Details: []template.HTML{ctx.Locale.Tr("repo.pulls.work_in_progress_desc")},
		}
	}
	if len(data.mergeBlockers) > 0 {
		data.BlockedSection = &pullMergeBoxSection{
			Icon: "octicon-alert-fill", IconClass: sectionColorDanger, Title: ctx.Locale.Tr("repo.pulls.merging_is_blocked"),
			Details: data.mergeBlockers, Files: pull.ChangedProtectedFiles,
		}
	}

	data.IsReady = data.hasPermToMerge && !prInfo.IsPullRequestBroken && pull.IsStatusMergeable() && data.WorkInProgressSection == nil && len(data.mergeBlockers) == 0 &&
		(data.StatusCheckData == nil || data.StatusCheckData.AllPassed())
	if data.MergeFormProps != nil {
		data.MergeFormProps["isReady"] = data.IsReady
	}
}
