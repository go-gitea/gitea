// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	pull_service "gitea.dev/services/pull"
)

func defaultMergeStyle(prConfig *repo_model.PullRequestsConfig) repo_model.MergeStyle {
	for _, style := range []repo_model.MergeStyle{
		prConfig.DefaultMergeStyle, repo_model.MergeStyleMerge, repo_model.MergeStyleRebase, repo_model.MergeStyleRebaseMerge,
		repo_model.MergeStyleSquash, repo_model.MergeStyleFastForwardOnly, repo_model.MergeStyleManuallyMerged,
	} {
		if prConfig.IsMergeStyleAllowed(style) {
			return style
		}
	}
	return ""
}

func (prInfo *pullRequestViewInfo) prepareMergeBoxFormProps(ctx *context.Context, prConfig *repo_model.PullRequestsConfig, gitRepo *git.Repository) {
	pull := prInfo.issue.PullRequest
	if !prInfo.MergeBoxData.hasPermToMerge || prInfo.MergeBoxData.AutoMerge != nil {
		return
	}
	mergeStyle := defaultMergeStyle(prConfig)
	if mergeStyle == "" {
		return
	}

	var defaultMergeTitle, defaultMergeBody string
	var defaultSquashMergeTitle, defaultSquashMergeBody string
	var defaultSquashMergeCommitMessages string
	if gitRepo != nil && !prInfo.IsPullRequestBroken && !pull.IsUnrelated() {
		var err error
		defaultMergeTitle, defaultMergeBody, err = pull_service.GetDefaultMergeMessage(ctx, gitRepo, pull, mergeStyle)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			log.Error("GetDefaultMergeMessage for style %s failed, error: %v", mergeStyle, err)
		}
		if prConfig.AllowSquash {
			if mergeStyle == repo_model.MergeStyleSquash {
				defaultSquashMergeTitle, defaultSquashMergeBody = defaultMergeTitle, defaultMergeBody
			} else {
				defaultSquashMergeTitle, defaultSquashMergeBody, err = pull_service.GetDefaultMergeMessage(ctx, gitRepo, pull, repo_model.MergeStyleSquash)
				if err != nil && !errors.Is(err, util.ErrNotExist) {
					log.Error("GetDefaultMergeMessage for squash failed, error: %v", err)
				}
			}
			defaultSquashMergeCommitMessages, err = pull_service.GetSquashMergeCommitMessages(ctx, pull)
			if err != nil && !errors.Is(err, util.ErrNotExist) {
				log.Error("GetSquashMergeCommitMessages failed, error: %v", err)
			}
		}
	}

	allOverridableChecksOk := !prInfo.MergeBoxData.hasOverridableBlockers
	mergeable := pull.IsStatusMergeable() || pull.IsEmpty()
	isMergeBlocked := prInfo.IsPullRequestBroken || prInfo.workInProgressPrefix != "" || prInfo.MergeBoxData.hasOpenDependencies
	canMergeNow := mergeable && !isMergeBlocked && (allOverridableChecksOk || prInfo.MergeBoxData.canBypassProtection)
	canAutoMerge := !isMergeBlocked && (mergeable || pull.IsFilesConflicted() || pull.IsChecking())
	mergeFormProps := map[string]any{
		"baseLink":             prInfo.issue.Link(),
		"textCancel":           ctx.Locale.Tr("cancel"),
		"textMergeCommitId":    ctx.Locale.Tr("repo.pulls.merge_commit_id"),
		"textSelectMergeStyle": ctx.Locale.Tr("repo.pulls.select_merge_style"),
		"textMergeTitle":       ctx.Locale.Tr("repo.pulls.merge_commit_title"),
		"textMergeBlocked":     ctx.Locale.Tr("repo.pulls.merge_blocked_by_requirements"),

		"isReady":                      prInfo.MergeBoxData.IsReady,
		"canMergeNow":                  canMergeNow,
		"allOverridableChecksOk":       allOverridableChecksOk,
		"textBypassRules":              ctx.Locale.Tr("repo.pulls.merge_bypass_rules"),
		"pullHeadCommitID":             prInfo.CompareInfo.HeadCommitID,
		"defaultMergeStyle":            mergeStyle,
		"mergeMessageFieldPlaceHolder": ctx.Locale.Tr("repo.editor.commit_message_desc"),

		"showPullCommands":      prInfo.MergeBoxData.ShowPullCommands,
		"showMergeInstructions": prInfo.MergeBoxData.ShowMergeInstructions,
		"textCmdMergeHint":      ctx.Locale.Tr("repo.pulls.cmd_instruction_merge_hint"),
		"textCmdHint":           ctx.Locale.Tr("repo.pulls.cmd_instruction_hint"),
	}

	var mergeStyles []map[string]any
	addMergeStyle := func(style repo_model.MergeStyle, allowed bool, textKey, mergeTitle, mergeMessage string) {
		if !allowed || prInfo.MergeBoxData.unsignable && style != repo_model.MergeStyleFastForwardOnly { // fast-forward-only creates no commit to sign
			return
		}
		short := ctx.Locale.Tr("repo.pulls.merge_style_short." + string(style))
		mergeStyles = append(mergeStyles, map[string]any{
			"name":                   style,
			"textDoMerge":            ctx.Locale.Tr(textKey),
			"textConfirmMerge":       ctx.Locale.Tr("repo.pulls.confirm_merge", short),
			"textDescription":        ctx.Locale.Tr("repo.pulls.merge_style_desc." + string(style)),
			"textAutoMerge":          ctx.Locale.Tr("repo.pulls.enable_auto_merge", short),
			"textConfirmAutoMerge":   ctx.Locale.Tr("repo.pulls.confirm_auto_merge", short),
			"textBypassMerge":        ctx.Locale.Tr("repo.pulls.bypass_rules_and_merge", short),
			"textConfirmBypassMerge": ctx.Locale.Tr("repo.pulls.confirm_bypass_rules_and_merge", short),
			"hideAutoMerge":          !canAutoMerge || canMergeNow && allOverridableChecksOk,
			"mergeTitleFieldText":    mergeTitle,
			"mergeMessageFieldText":  mergeMessage,
			"hideMergeMessageTexts":  style == repo_model.MergeStyleRebase || style == repo_model.MergeStyleFastForwardOnly,
		})
	}
	addMergeStyle(repo_model.MergeStyleMerge, prConfig.AllowMerge, "repo.pulls.merge_pull_request", defaultMergeTitle, defaultMergeBody)
	addMergeStyle(repo_model.MergeStyleRebase, prConfig.AllowRebase, "repo.pulls.rebase_merge_pull_request", "", "")
	addMergeStyle(repo_model.MergeStyleRebaseMerge, prConfig.AllowRebaseMerge, "repo.pulls.rebase_merge_commit_pull_request", defaultMergeTitle, defaultMergeBody)
	addMergeStyle(repo_model.MergeStyleSquash, prConfig.AllowSquash, "repo.pulls.squash_merge_pull_request", defaultSquashMergeTitle, git.CommitMessageMerge(defaultSquashMergeCommitMessages, defaultSquashMergeBody))
	addMergeStyle(repo_model.MergeStyleFastForwardOnly, prConfig.AllowFastForwardOnly && pull.CommitsBehind == 0, "repo.pulls.fast_forward_only_merge_pull_request", "", "")

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
			"textDoMerge":           ctx.Locale.Tr("repo.pulls.merge_manually"),
			"textDescription":       ctx.Locale.Tr("repo.pulls.merge_style_desc.manually-merged"),
			"hideMergeMessageTexts": true,
			"hideAutoMerge":         true,
		})
	}

	if len(mergeStyles) > 0 {
		mergeFormProps["mergeStyles"] = mergeStyles
		prInfo.MergeBoxData.MergeFormProps = mergeFormProps
	}
}
