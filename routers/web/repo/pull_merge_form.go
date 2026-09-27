// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"maps"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	pull_service "gitea.dev/services/pull"
)

func (prInfo *pullRequestViewInfo) prepareMergeBoxFormProps(ctx *context.Context, prConfig *repo_model.PullRequestsConfig) {
	pull := prInfo.issue.PullRequest
	if pull.HasMerged || prInfo.issue.IsClosed {
		return
	}
	if !prInfo.MergeBoxData.hasPermToMerge || prInfo.MergeBoxData.AutoMerge != nil {
		return
	}
	prInfo.MergeBoxData.ShowPullCommands = pull.HeadRepo != nil

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
		if pull.IsStatusMergeable() {
			prInfo.MergeBoxData.mergeBlockers = append(prInfo.MergeBoxData.mergeBlockers, ctx.Locale.Tr("repo.pulls.no_merge_desc"), ctx.Locale.Tr("repo.pulls.no_merge_helper"))
		}
		return
	}

	var defaultMergeTitle, defaultMergeBody string
	var defaultSquashMergeTitle, defaultSquashMergeBody string
	var defaultSquashMergeCommitMessages string
	if !prInfo.IsPullRequestBroken && ctx.Repo.GitRepo != nil { // the devtest page has no git repo
		var err error
		defaultMergeTitle, defaultMergeBody, err = pull_service.GetDefaultMergeMessage(ctx, ctx.Repo.GitRepo, pull, mergeStyle)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			log.Error("GetDefaultMergeMessage for style %s failed, error: %v", mergeStyle, err)
		}
		defaultSquashMergeTitle, defaultSquashMergeBody, err = pull_service.GetDefaultMergeMessage(ctx, ctx.Repo.GitRepo, pull, repo_model.MergeStyleSquash)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			log.Error("GetDefaultMergeMessage for squash failed, error: %v", err)
		}
		defaultSquashMergeCommitMessages, err = pull_service.GetSquashMergeCommitMessages(ctx, pull)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			log.Error("GetSquashMergeCommitMessages failed, error: %v", err)
		}
	}

	allOverridableChecksOk := !prInfo.MergeBoxData.hasOverridableBlockers
	mergeFormProps := map[string]any{
		"baseLink":             prInfo.issue.Link(),
		"textCancel":           ctx.Locale.Tr("cancel"),
		"textDeleteBranch":     ctx.Locale.Tr("repo.branch.delete", prInfo.headTarget),
		"textMergeCommitId":    ctx.Locale.Tr("repo.pulls.merge_commit_id"),
		"textSelectMergeStyle": ctx.Locale.Tr("repo.pulls.select_merge_style"),
		"textMergeTitle":       ctx.Locale.Tr("repo.pulls.merge_commit_title"),

		"canMergeNow":                   prInfo.MergeBoxData.canMergeNow,
		"allOverridableChecksOk":        allOverridableChecksOk,
		"textBypassRules":               ctx.Locale.Tr("repo.pulls.merge_bypass_rules"),
		"pullHeadCommitID":              prInfo.CompareInfo.HeadCommitID,
		"isPullBranchDeletable":         prInfo.MergeBoxData.IsPullBranchDeletable,
		"defaultMergeStyle":             mergeStyle,
		"defaultDeleteBranchAfterMerge": prConfig.DefaultDeleteBranchAfterMerge,
		"mergeMessageFieldPlaceHolder":  ctx.Locale.Tr("repo.editor.commit_message_desc"),

		"showPullCommands": prInfo.MergeBoxData.ShowPullCommands,
		"textCmdMergeHint": ctx.Locale.Tr("repo.pulls.cmd_instruction_merge_hint"),
		"textCmdHint":      ctx.Locale.Tr("repo.pulls.cmd_instruction_hint"),
	}

	// if this pr can be merged now, then hide the auto merge
	generalHideAutoMerge := prInfo.MergeBoxData.canMergeNow && allOverridableChecksOk
	var mergeStyles []map[string]any
	addMergeStyle := func(style repo_model.MergeStyle, allowed bool, textKey string, fields map[string]any) {
		if !allowed || prInfo.MergeBoxData.unsignable && style != repo_model.MergeStyleFastForwardOnly { // fast-forward-only creates no commit to sign
			return
		}
		short := ctx.Locale.Tr("repo.pulls.merge_style_short." + string(style))
		maps.Copy(fields, map[string]any{
			"name":                   style,
			"textDoMerge":            ctx.Locale.Tr("repo.pulls." + textKey),
			"textConfirmMerge":       ctx.Locale.Tr("repo.pulls.confirm_merge", short),
			"textDescription":        ctx.Locale.Tr("repo.pulls.merge_style_desc." + string(style)),
			"textAutoMerge":          ctx.Locale.Tr("repo.pulls.enable_auto_merge", short),
			"textConfirmAutoMerge":   ctx.Locale.Tr("repo.pulls.confirm_auto_merge", short),
			"textBypassMerge":        ctx.Locale.Tr("repo.pulls.bypass_rules_and_merge", short),
			"textConfirmBypassMerge": ctx.Locale.Tr("repo.pulls.confirm_bypass_rules_and_merge", short),
			"hideAutoMerge":          generalHideAutoMerge,
		})
		mergeStyles = append(mergeStyles, fields)
	}
	if (pull.IsStatusMergeable() || pull.IsEmpty()) && !prInfo.MergeBoxData.isMergeBlocked {
		addMergeStyle(repo_model.MergeStyleMerge, prConfig.AllowMerge, "merge_pull_request", map[string]any{
			"mergeTitleFieldText":   defaultMergeTitle,
			"mergeMessageFieldText": defaultMergeBody,
		})
		addMergeStyle(repo_model.MergeStyleRebase, prConfig.AllowRebase, "rebase_merge_pull_request", map[string]any{
			"hideMergeMessageTexts": true,
		})
		addMergeStyle(repo_model.MergeStyleRebaseMerge, prConfig.AllowRebaseMerge, "rebase_merge_commit_pull_request", map[string]any{
			"mergeTitleFieldText":   defaultMergeTitle,
			"mergeMessageFieldText": defaultMergeBody,
		})
		addMergeStyle(repo_model.MergeStyleSquash, prConfig.AllowSquash, "squash_merge_pull_request", map[string]any{
			"mergeTitleFieldText":   defaultSquashMergeTitle,
			"mergeMessageFieldText": git.CommitMessageMerge(defaultSquashMergeCommitMessages, defaultSquashMergeBody),
		})
		addMergeStyle(repo_model.MergeStyleFastForwardOnly, prConfig.AllowFastForwardOnly && pull.CommitsBehind == 0, "fast_forward_only_merge_pull_request", map[string]any{
			"hideMergeMessageTexts": true,
		})
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
