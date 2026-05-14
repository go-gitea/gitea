// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"html/template"

	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	pull_service "code.gitea.io/gitea/services/pull"
)

func (prInfo *pullRequestViewInfo) prepareMergeBoxFormProps(ctx *context.Context) {
	pull := prInfo.issue.PullRequest
	if pull.HasMerged || prInfo.issue.IsClosed {
		return
	}
	if !prInfo.MergeBoxData.allowMerge {
		return
	}

	prConfig := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypePullRequests).PullRequestsConfig()

	// Check correct values and select default
	var mergeStyle repo_model.MergeStyle
	if prConfig.IsMergeStyleAllowed(prConfig.DefaultMergeStyle) {
		mergeStyle = prConfig.DefaultMergeStyle
	} else if prConfig.AllowMerge {
		mergeStyle = repo_model.MergeStyleMerge
	} else if prConfig.AllowRebase {
		mergeStyle = repo_model.MergeStyleRebase
	} else if prConfig.AllowRebaseMerge {
		mergeStyle = repo_model.MergeStyleRebaseMerge
	} else if prConfig.AllowSquash {
		mergeStyle = repo_model.MergeStyleSquash
	} else if prConfig.AllowFastForwardOnly {
		mergeStyle = repo_model.MergeStyleFastForwardOnly
	} else if prConfig.AllowManualMerge {
		mergeStyle = repo_model.MergeStyleManuallyMerged
	}
	if mergeStyle == "" {
		return
	}

	// Check if there is a pending pr merge
	hasPendingPullRequestMerge, pendingPullRequestMerge, err := pull_model.GetScheduledMergeByPullID(ctx, pull.ID)
	if err != nil {
		ctx.ServerError("GetScheduledMergeByPullID", err)
		return
	}

	var hasPendingPullRequestMergeTip template.HTML
	if hasPendingPullRequestMerge {
		createdPRMergeStr := templates.TimeSince(pendingPullRequestMerge.CreatedUnix)
		hasPendingPullRequestMergeTip = ctx.Locale.Tr("repo.pulls.auto_merge_has_pending_schedule", pendingPullRequestMerge.Doer.Name, createdPRMergeStr)
	}

	defaultMergeTitle, defaultMergeBody, err := pull_service.GetDefaultMergeMessage(ctx, ctx.Repo.GitRepo, pull, mergeStyle)
	if err != nil {
		ctx.ServerError("GetDefaultMergeMessage", err)
		return
	}
	defaultSquashMergeTitle, defaultSquashMergeBody, err := pull_service.GetDefaultMergeMessage(ctx, ctx.Repo.GitRepo, pull, repo_model.MergeStyleSquash)
	if err != nil {
		ctx.ServerError("GetDefaultSquashMergeMessage", err)
		return
	}

	var defaultSquashMergeCommitMessages string
	if !prInfo.IsPullRequestBroken {
		defaultSquashMergeCommitMessages = pull_service.GetSquashMergeCommitMessages(ctx, pull)
	}

	allOverridableChecksOk := !prInfo.MergeBoxData.HasOverridableBlockers
	mergeFormProps := map[string]any{
		"baseLink":                       prInfo.issue.Link(),
		"textCancel":                     ctx.Locale.Tr("cancel"),
		"textDeleteBranch":               ctx.Locale.Tr("repo.branch.delete", prInfo.headTarget),
		"textAutoMergeButtonWhenSucceed": ctx.Locale.Tr("repo.pulls.auto_merge_button_when_succeed"),
		"textAutoMergeWhenSucceed":       ctx.Locale.Tr("repo.pulls.auto_merge_when_succeed"),
		"textAutoMergeCancelSchedule":    ctx.Locale.Tr("repo.pulls.auto_merge_cancel_schedule"),
		"textClearMergeMessage":          ctx.Locale.Tr("repo.pulls.clear_merge_message"),
		"textClearMergeMessageHint":      ctx.Locale.Tr("repo.pulls.clear_merge_message_hint"),
		"textMergeCommitId":              ctx.Locale.Tr("repo.pulls.merge_commit_id"),

		"canMergeNow":                   prInfo.MergeBoxData.CanMergeNow,
		"allOverridableChecksOk":        allOverridableChecksOk,
		"emptyCommit":                   pull.IsEmpty(),
		"pullHeadCommitID":              prInfo.CompareInfo.HeadCommitID,
		"isPullBranchDeletable":         prInfo.MergeBoxData.IsPullBranchDeletable,
		"defaultMergeStyle":             mergeStyle,
		"defaultDeleteBranchAfterMerge": prConfig.DefaultDeleteBranchAfterMerge,
		"mergeMessageFieldPlaceHolder":  ctx.Locale.Tr("repo.editor.commit_message_desc"),
		"defaultMergeMessage":           defaultMergeBody,

		"hasPendingPullRequestMerge":    hasPendingPullRequestMerge,
		"hasPendingPullRequestMergeTip": hasPendingPullRequestMergeTip,
	}

	// if this pr can be merged now, then hide the auto merge
	generalHideAutoMerge := prInfo.MergeBoxData.CanMergeNow && allOverridableChecksOk

	var mergeStyles []any
	if pull.IsStatusMergeable() {
		mergeStyles = []any{
			map[string]any{
				"name":                  "merge",
				"allowed":               prConfig.AllowMerge,
				"textDoMerge":           ctx.Locale.Tr("repo.pulls.merge_pull_request"),
				"mergeTitleFieldText":   defaultMergeTitle,
				"mergeMessageFieldText": defaultMergeBody,
				"hideAutoMerge":         generalHideAutoMerge,
			},
			map[string]any{
				"name":                  "rebase",
				"allowed":               prConfig.AllowRebase,
				"textDoMerge":           ctx.Locale.Tr("repo.pulls.rebase_merge_pull_request"),
				"hideMergeMessageTexts": true,
				"hideAutoMerge":         generalHideAutoMerge,
			},
			map[string]any{
				"name":                  "rebase-merge",
				"allowed":               prConfig.AllowRebaseMerge,
				"textDoMerge":           ctx.Locale.Tr("repo.pulls.rebase_merge_commit_pull_request"),
				"mergeTitleFieldText":   defaultMergeTitle,
				"mergeMessageFieldText": defaultMergeBody,
				"hideAutoMerge":         generalHideAutoMerge,
			},
			map[string]any{
				"name":                  "squash",
				"allowed":               prConfig.AllowSquash,
				"textDoMerge":           ctx.Locale.Tr("repo.pulls.squash_merge_pull_request"),
				"mergeTitleFieldText":   defaultSquashMergeTitle,
				"mergeMessageFieldText": defaultSquashMergeCommitMessages + defaultSquashMergeBody,
				"hideAutoMerge":         generalHideAutoMerge,
			},
			map[string]any{
				"name":                  "fast-forward-only",
				"allowed":               prConfig.AllowFastForwardOnly && pull.CommitsBehind == 0,
				"textDoMerge":           ctx.Locale.Tr("repo.pulls.fast_forward_only_merge_pull_request"),
				"hideMergeMessageTexts": true,
				"hideAutoMerge":         generalHideAutoMerge,
			},
		}
	}

	// Manually Merged is not a well-known feature, it is used to mark a non-mergeable PR (already merged, conflicted) as merged
	// To test it:
	//  Enable "Manually Merged" feature in the Repository Settings
	//  Create a pull request, either:
	//  - Merge the pull request branch locally and push the merged commit to Gitea
	//  - Make some conflicts between the base branch and the pull request branch
	//  Then the Manually Merged form will be shown in the merge form
	canUseManualMerge := !pull.IsWorkInProgress(ctx) && !pull.IsChecking() && prConfig.AllowManualMerge
	if canUseManualMerge {
		mergeStyles = append(mergeStyles, map[string]any{
			"name":                  "manually-merged",
			"allowed":               prConfig.AllowManualMerge,
			"textDoMerge":           ctx.Locale.Tr("repo.pulls.merge_manually"),
			"hideMergeMessageTexts": true,
			"hideAutoMerge":         true,
		})
	}

	if len(mergeStyles) > 0 {
		mergeFormProps["mergeStyles"] = mergeStyles
		prInfo.MergeBoxData.MergeFormProps = mergeFormProps
	} else if pull.IsStatusMergeable() {
		// no merge style was set in repo setting
		prInfo.MergeBoxData.infoCommitBlockers.AddInfoItem(
			svg.RenderHTML("octicon-x", 16, "tw-text-red"),
			ctx.Locale.Tr("repo.pulls.no_merge_desc"),
		)
		prInfo.MergeBoxData.infoCommitBlockers.AddInfoItem(
			svg.RenderHTML("octicon-info"),
			ctx.Locale.Tr("repo.pulls.no_merge_helper"),
		)
	}
}
