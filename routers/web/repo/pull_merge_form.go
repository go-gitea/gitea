// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"html/template"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/translation"
	"gitea.dev/modules/util"
	asymkey_service "gitea.dev/services/asymkey"
	"gitea.dev/services/context"
	pull_service "gitea.dev/services/pull"
)

var mergeStyleTrKeys = map[repo_model.MergeStyle]struct{ shortName, description string }{
	repo_model.MergeStyleMerge:           {"repo.pulls.merge_style_short.merge", "repo.pulls.merge_style_desc.merge"},
	repo_model.MergeStyleRebase:          {"repo.pulls.merge_style_short.rebase", "repo.pulls.merge_style_desc.rebase"},
	repo_model.MergeStyleRebaseMerge:     {"repo.pulls.merge_style_short.rebase-merge", "repo.pulls.merge_style_desc.rebase-merge"},
	repo_model.MergeStyleSquash:          {"repo.pulls.merge_style_short.squash", "repo.pulls.merge_style_desc.squash"},
	repo_model.MergeStyleFastForwardOnly: {"repo.pulls.merge_style_short.fast-forward-only", "repo.pulls.merge_style_desc.fast-forward-only"},
}

func trMergeStyleShortName(locale translation.Locale, style repo_model.MergeStyle) template.HTML {
	if keys, ok := mergeStyleTrKeys[style]; ok {
		return locale.Tr(keys.shortName)
	}
	return ""
}

var autoMergeDisabledReasonTrKeys = map[string]string{
	"closed":               "repo.pulls.auto_merge_disabled_reason.closed",
	"pushed_by_non_writer": "repo.pulls.auto_merge_disabled_reason.pushed_by_non_writer",
	"base_changed":         "repo.pulls.auto_merge_disabled_reason.base_changed",
	"work_in_progress":     "repo.pulls.auto_merge_disabled_reason.work_in_progress",
}

func trAutoMergeDisabledReason(locale translation.Locale, reason string) template.HTML {
	if key, ok := autoMergeDisabledReasonTrKeys[reason]; ok {
		return locale.Tr(key)
	}
	return ""
}

type pullMergeStyleProps struct {
	Name                   repo_model.MergeStyle `json:"name"`
	TextDoMerge            template.HTML         `json:"textDoMerge"`
	TextDescription        template.HTML         `json:"textDescription"`
	TextConfirmMerge       template.HTML         `json:"textConfirmMerge,omitempty"`
	TextAutoMerge          template.HTML         `json:"textAutoMerge,omitempty"`
	TextConfirmAutoMerge   template.HTML         `json:"textConfirmAutoMerge,omitempty"`
	TextBypassMerge        template.HTML         `json:"textBypassMerge,omitempty"`
	TextConfirmBypassMerge template.HTML         `json:"textConfirmBypassMerge,omitempty"`
	MergeTitleFieldText    string                `json:"mergeTitleFieldText,omitempty"`
	MergeMessageFieldText  string                `json:"mergeMessageFieldText,omitempty"`
	HideMergeMessageTexts  bool                  `json:"hideMergeMessageTexts"`
}

func (prInfo *pullRequestViewInfo) prepareMergeBoxFormProps(ctx *context.Context, prConfig *repo_model.PullRequestsConfig, gitRepo *git.Repository) {
	pull := prInfo.issue.PullRequest
	data := prInfo.MergeBoxData
	var mergeStyles []*pullMergeStyleProps
	var mergeStyle repo_model.MergeStyle
	addMergeStyle := func(props *pullMergeStyleProps) {
		if mergeStyle == "" || props.Name == prConfig.DefaultMergeStyle {
			mergeStyle = props.Name
		}
		mergeStyles = append(mergeStyles, props)
	}
	hasEnabledStyle, hasUnsignableStyle := prConfig.AllowManualMerge, false
	addRegularMergeStyle := func(style repo_model.MergeStyle, allowed bool, textKey string) {
		hasEnabledStyle = hasEnabledStyle || allowed
		if !allowed || pull.IsAncestor() || style == repo_model.MergeStyleFastForwardOnly && pull.CommitsBehind > 0 {
			return
		}
		if data.mergeSigningError != nil && style != repo_model.MergeStyleFastForwardOnly { // fast-forward-only creates no commit to sign
			hasUnsignableStyle = true
			return
		}
		short := trMergeStyleShortName(ctx.Locale, style)
		addMergeStyle(&pullMergeStyleProps{
			Name:                   style,
			TextDoMerge:            ctx.Locale.Tr(textKey),
			TextDescription:        ctx.Locale.Tr(mergeStyleTrKeys[style].description),
			TextConfirmMerge:       ctx.Locale.Tr("repo.pulls.confirm_merge", short),
			TextAutoMerge:          ctx.Locale.Tr("repo.pulls.enable_auto_merge", short),
			TextConfirmAutoMerge:   ctx.Locale.Tr("repo.pulls.confirm_auto_merge", short),
			TextBypassMerge:        ctx.Locale.Tr("repo.pulls.bypass_rules_and_merge", short),
			TextConfirmBypassMerge: ctx.Locale.Tr("repo.pulls.confirm_bypass_rules_and_merge", short),
			HideMergeMessageTexts:  style == repo_model.MergeStyleRebase || style == repo_model.MergeStyleFastForwardOnly,
		})
	}
	addRegularMergeStyle(repo_model.MergeStyleMerge, prConfig.AllowMerge, "repo.pulls.merge_pull_request")
	addRegularMergeStyle(repo_model.MergeStyleRebase, prConfig.AllowRebase, "repo.pulls.rebase_merge_pull_request")
	addRegularMergeStyle(repo_model.MergeStyleRebaseMerge, prConfig.AllowRebaseMerge, "repo.pulls.rebase_merge_commit_pull_request")
	addRegularMergeStyle(repo_model.MergeStyleSquash, prConfig.AllowSquash, "repo.pulls.squash_merge_pull_request")
	addRegularMergeStyle(repo_model.MergeStyleFastForwardOnly, prConfig.AllowFastForwardOnly, "repo.pulls.fast_forward_only_merge_pull_request")

	mergeable := pull.IsStatusMergeable()
	canAttemptMerge := data.hasPermToMerge && len(mergeStyles) > 0 && !prInfo.IsPullRequestBroken && prInfo.workInProgressPrefix == "" && !data.hasOpenDependencies
	canMergeNow := canAttemptMerge && mergeable && (!data.hasOverridableBlockers || data.canBypassProtection)
	canAutoMerge := canAttemptMerge && (mergeable || pull.IsFilesConflicted() || pull.IsChecking())
	data.IsMergeStateClean = canMergeNow && !data.hasOverridableBlockers && !pull.IsEmpty() && (data.StatusCheckData == nil || data.StatusCheckData.AllPassed())
	if len(mergeStyles) == 0 && hasUnsignableStyle {
		data.mergeBlockers = append(data.mergeBlockers, ctx.Locale.Tr("repo.pulls.require_signed_wont_sign"))
		if errWontSign, ok := data.mergeSigningError.(*asymkey_service.ErrWontSign); ok {
			data.mergeBlockers = append(data.mergeBlockers, ctx.Locale.Tr("repo.signing.wont_sign."+string(errWontSign.Reason)))
		}
	}

	// Manually Merged is not a well-known feature, it is used to mark a non-mergeable PR (already merged, conflicted) as merged
	// To test it:
	//  Enable "Manually Merged" feature in the Repository Settings
	//  Create a pull request, either:
	//  - Merge the pull request branch locally and push the merged commit to Gitea
	//  - Make some conflicts between the base branch and the pull request branch
	//  Then the Manually Merged form will be shown in the merge form
	if prInfo.workInProgressPrefix == "" && !pull.IsChecking() && prConfig.AllowManualMerge {
		addMergeStyle(&pullMergeStyleProps{
			Name:                  repo_model.MergeStyleManuallyMerged,
			TextDoMerge:           ctx.Locale.Tr("repo.pulls.merge_manually"),
			TextDescription:       ctx.Locale.Tr("repo.pulls.merge_style_desc.manually-merged"),
			HideMergeMessageTexts: true,
		})
	}

	if !data.hasPermToMerge || data.AutoMerge != nil {
		return
	}
	if len(mergeStyles) == 0 {
		if mergeable && !hasEnabledStyle {
			data.mergeBlockers = append(data.mergeBlockers, ctx.Locale.Tr("repo.pulls.no_merge_desc"), ctx.Locale.Tr("repo.pulls.no_merge_helper"))
		}
		return
	}
	if gitRepo != nil && !prInfo.IsPullRequestBroken && !pull.IsUnrelated() {
		defaultMergeTitle, defaultMergeBody, err := pull_service.GetDefaultMergeMessage(ctx, gitRepo, pull, mergeStyle)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			log.Error("GetDefaultMergeMessage for style %s failed, error: %v", mergeStyle, err)
		}
		for _, style := range mergeStyles {
			if style.HideMergeMessageTexts {
				continue
			}
			style.MergeTitleFieldText, style.MergeMessageFieldText = defaultMergeTitle, defaultMergeBody
			if style.Name != repo_model.MergeStyleSquash {
				continue
			}
			if mergeStyle != repo_model.MergeStyleSquash {
				style.MergeTitleFieldText, style.MergeMessageFieldText, err = pull_service.GetDefaultMergeMessage(ctx, gitRepo, pull, repo_model.MergeStyleSquash)
				if err != nil && !errors.Is(err, util.ErrNotExist) {
					log.Error("GetDefaultMergeMessage for squash failed, error: %v", err)
				}
			}
			squashCommitMessages, err := pull_service.GetSquashMergeCommitMessages(ctx, pull)
			if err != nil && !errors.Is(err, util.ErrNotExist) {
				log.Error("GetSquashMergeCommitMessages failed, error: %v", err)
			}
			style.MergeMessageFieldText = git.CommitMessageMerge(squashCommitMessages, style.MergeMessageFieldText)
		}
	}

	data.MergeFormProps = map[string]any{
		"baseLink":             prInfo.issue.Link(),
		"textCancel":           ctx.Locale.Tr("cancel"),
		"textMergeCommitId":    ctx.Locale.Tr("repo.pulls.merge_commit_id"),
		"textSelectMergeStyle": ctx.Locale.Tr("repo.pulls.select_merge_style"),
		"textMergeTitle":       ctx.Locale.Tr("form.CommitMessage"),
		"textMergeBlocked":     ctx.Locale.Tr("repo.pulls.merging_is_blocked"),

		"isMergeStateClean":            data.IsMergeStateClean,
		"canMergeNow":                  canMergeNow,
		"showAutoMerge":                canAutoMerge && (!canMergeNow || data.hasOverridableBlockers),
		"canBypassMerge":               canMergeNow && (data.hasOverridableBlockers || data.MergeQueueLink != "" && data.canBypassProtection),
		"textBypassRules":              ctx.Locale.Tr("repo.pulls.merge_bypass_rules"),
		"pullHeadCommitID":             prInfo.CompareInfo.HeadCommitID,
		"defaultMergeStyle":            mergeStyle,
		"mergeStyles":                  mergeStyles,
		"mergeMessageFieldPlaceHolder": ctx.Locale.Tr("repo.editor.commit_message_desc"),
		"textCmdMergeHint":             util.Iif(data.ShowMergeInstructions, ctx.Locale.Tr("repo.pulls.cmd_instruction_merge_hint"), ""),
		"textCmdInstructions":          util.Iif(data.ShowPullCommands, ctx.Locale.Tr("repo.pulls.cmd_instruction_hint"), ""),
	}
	if data.MergeQueueLink != "" {
		data.MergeFormProps["queue"] = map[string]any{
			"style":    prInfo.ProtectedBranchRule.MergeQueue.MergeMethod,
			"button":   ctx.Tr("repo.pulls.merge_queue.merge_when_ready"),
			"confirm":  ctx.Tr("repo.pulls.merge_queue.confirm_merge_when_ready"),
			"help":     ctx.Locale.TrString("repo.pulls.merge_queue.help"),
			"linkText": ctx.Locale.TrString("repo.pulls.merge_queue.link_text"),
			"link":     data.MergeQueueLink,
			"branch":   pull.BaseBranch,
		}
	}
}
