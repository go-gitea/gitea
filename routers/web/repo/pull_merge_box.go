// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"html/template"

	"code.gitea.io/gitea/modules/htmlutil"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

type pullMergeBoxInfoItem struct {
	ItemClass   string
	SvgIconHTML template.HTML
	InfoHTML    template.HTML
	ListItems   []template.HTML
}

type pullMergeBoxInfoItemCollection struct {
	items []*pullMergeBoxInfoItem
}

type pullInfoSection struct {
	InfoItems []*pullMergeBoxInfoItem
}

func escapeStringSliceToHTML(s []string) (ret []template.HTML) {
	for _, v := range s {
		ret = append(ret, template.HTML(template.HTMLEscapeString(v)))
	}
	return ret
}

func (c *pullMergeBoxInfoItemCollection) AddInfoItem(svg, info template.HTML, optItems ...[]template.HTML) {
	c.items = append(c.items, &pullMergeBoxInfoItem{
		SvgIconHTML: svg,
		InfoHTML:    info,
		ListItems:   util.OptionalArg(optItems),
	})
}

func (c *pullMergeBoxInfoItemCollection) AddErrorItem(svg, info template.HTML, optItems ...[]template.HTML) {
	c.items = append(c.items, &pullMergeBoxInfoItem{
		ItemClass:   "tw-text-red",
		SvgIconHTML: svg,
		InfoHTML:    info,
		ListItems:   util.OptionalArg(optItems),
	})
}

func (prInfo *pullRequestViewInfo) prepareMergeBoxIconColor() {
	pull := prInfo.issue.PullRequest
	mergeBoxData := prInfo.MergeBoxData

	showAsNormalColor := prInfo.issue.IsClosed || prInfo.workInProgressPrefix != "" || pull.IsEmpty() || pull.IsFilesConflicted()
	showAsErrorColor := false
	showAsWarningColor := pull.IsChecking()

	if statusCheckData := mergeBoxData.StatusCheckData; statusCheckData != nil {
		showAsErrorColor = statusCheckData.pullCommitStatusState.IsError() || statusCheckData.pullCommitStatusState.IsFailure() ||
			statusCheckData.RequiredChecksState.IsError() || statusCheckData.RequiredChecksState.IsFailure()

		showAsWarningColor = showAsWarningColor ||
			statusCheckData.pullCommitStatusState.IsWarning() || statusCheckData.pullCommitStatusState.IsPending() ||
			(mergeBoxData.enableStatusCheck && (statusCheckData.RequiredChecksState.IsWarning() || statusCheckData.RequiredChecksState.IsPending()))
	}

	hasBlockers := len(mergeBoxData.infoCommitBlockers.items) > 0 || len(mergeBoxData.infoProtectionBlockers.items) > 0

	switch {
	case pull.HasMerged:
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-purple"
	case showAsNormalColor:
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-text-light"
	case showAsErrorColor:
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-red"
	case showAsWarningColor:
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-yellow"
	case hasBlockers:
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-red"
	case pull.IsStatusMergeable():
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-green"
	default:
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-text-light"
	}
}

func (prInfo *pullRequestViewInfo) prepareMergeBoxInfoItems(ctx *context.Context) {
	pull := prInfo.issue.PullRequest
	data := prInfo.MergeBoxData

	if pull.HasMerged && data.IsPullBranchDeletable {
		data.ClosedInfoTitle = ctx.Locale.Tr("repo.pulls.merged_success")
		data.ClosedInfoBody = ctx.Locale.Tr("repo.pulls.merged_info_text", htmlutil.HTMLFormat("<code>%s</code>", prInfo.headTarget))
		return
	} else if prInfo.issue.IsClosed {
		data.ClosedInfoTitle = ctx.Locale.Tr("repo.pulls.closed")
		if prInfo.IsPullRequestBroken {
			data.ClosedInfoBody = ctx.Locale.Tr("repo.pulls.cant_reopen_deleted_branch")
		} else {
			data.ClosedInfoBody = ctx.Locale.Tr("repo.pulls.reopen_to_merge")
		}
		return
	}

	if pull.IsFilesConflicted() {
		detailItems := escapeStringSliceToHTML(pull.ConflictedFiles)
		if len(detailItems) == 0 {
			detailItems = append(detailItems, ctx.Locale.Tr("repo.pulls.files_conflicted_no_listed_files"))
		}
		if len(detailItems) > 10 {
			detailItems = detailItems[:10]
			detailItems = append(detailItems, "...")
		}
		prInfo.MergeBoxData.infoCommitBlockers.AddInfoItem(
			svg.RenderHTML("octicon-x"),
			ctx.Locale.Tr("repo.pulls.files_conflicted"),
			detailItems,
		)
	}

	if prInfo.IsPullRequestBroken {
		prInfo.MergeBoxData.infoCommitBlockers.AddInfoItem(
			svg.RenderHTML("octicon-x"),
			ctx.Locale.Tr("repo.pulls.data_broken"),
		)
	}

	if pull.IsChecking() {
		prInfo.MergeBoxData.infoCommitBlockers.AddInfoItem(
			svg.RenderHTML("gitea-running", 16, "rotate-clockwise"),
			ctx.Locale.Tr("repo.pulls.is_checking"),
		)
	}

	if pull.IsAncestor() {
		prInfo.MergeBoxData.infoCommitBlockers.AddInfoItem(
			svg.RenderHTML("octicon-alert"),
			ctx.Locale.Tr("repo.pulls.is_ancestor"),
		)
	}

	if !pull.IsStatusMergeable() {
		// it is only a "protection" level blocker, it can be bypassed by admin (e.g.: manually merged)
		if pull.IsEmpty() {
			prInfo.MergeBoxData.infoProtectionBlockers.AddInfoItem(
				svg.RenderHTML("octicon-alert"),
				ctx.Locale.Tr("repo.pulls.is_empty"),
			)
		} else {
			prInfo.MergeBoxData.infoProtectionBlockers.AddErrorItem(
				svg.RenderHTML("octicon-x"),
				ctx.Locale.Tr("repo.pulls.cannot_auto_merge_desc"),
			)
			prInfo.MergeBoxData.infoProtectionBlockers.AddInfoItem(
				svg.RenderHTML("octicon-info"),
				ctx.Locale.Tr("repo.pulls.cannot_auto_merge_helper"),
			)
		}
	}

	if !data.allowMerge {
		prInfo.MergeBoxData.infoProtectionBlockers.AddInfoItem(
			svg.RenderHTML("octicon-info"),
			ctx.Locale.Tr("repo.pulls.no_merge_access"),
		)
	}

	if data.CanMergeNow {
		if data.HasOverridableBlockers {
			prInfo.MergeBoxData.infoMergePrompts.AddInfoItem(
				svg.RenderHTML("octicon-dot-fill"),
				ctx.Locale.Tr("repo.pulls.required_status_check_administrator"),
			)
		} else if pull.IsStatusMergeable() || pull.IsEmpty() {
			prInfo.MergeBoxData.infoMergePrompts.AddInfoItem(
				svg.RenderHTML("octicon-check"),
				ctx.Locale.Tr("repo.pulls.can_auto_merge_desc"),
			)
		}
	}

	if len(data.infoCommitBlockers.items) > 0 {
		data.InfoSections = append(data.InfoSections, &pullInfoSection{data.infoCommitBlockers.items})
	} else {
		data.InfoSections = append(data.InfoSections, &pullInfoSection{data.infoProtectionBlockers.items})
	}
	data.InfoSections = append(data.InfoSections, &pullInfoSection{data.infoMergePrompts.items})
}
