// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"html/template"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
	asymkey_service "gitea.dev/services/asymkey"
	"gitea.dev/services/context"
	pull_service "gitea.dev/services/pull"
)

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
		short := ctx.Locale.Tr("repo.pulls.merge_style_short." + string(style))
		addMergeStyle(&pullMergeStyleProps{
			Name:                   style,
			TextDoMerge:            ctx.Locale.Tr(textKey),
			TextDescription:        ctx.Locale.Tr("repo.pulls.merge_style_desc." + string(style)),
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
	data.IsReady = canMergeNow && !data.hasOverridableBlockers && !pull.IsEmpty() && (data.StatusCheckData == nil || data.StatusCheckData.AllPassed())
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

	for _, style := range mergeStyles {
		if style.Name == repo_model.MergeStyleSquash {
			style.MergeTitleFieldText, style.MergeMessageFieldText = defaultSquashMergeTitle, git.CommitMessageMerge(defaultSquashMergeCommitMessages, defaultSquashMergeBody)
		} else if !style.HideMergeMessageTexts {
			style.MergeTitleFieldText, style.MergeMessageFieldText = defaultMergeTitle, defaultMergeBody
		}
	}

	data.MergeFormProps = map[string]any{
		"baseLink":             prInfo.issue.Link(),
		"textCancel":           ctx.Locale.Tr("cancel"),
		"textMergeCommitId":    ctx.Locale.Tr("repo.pulls.merge_commit_id"),
		"textSelectMergeStyle": ctx.Locale.Tr("repo.pulls.select_merge_style"),
		"textMergeTitle":       ctx.Locale.Tr("form.CommitMessage"),
		"textMergeBlocked":     ctx.Locale.Tr("repo.pulls.merge_blocked_by_requirements"),

		"isReady":                      data.IsReady,
		"canMergeNow":                  canMergeNow,
		"canAutoMerge":                 canAutoMerge && (!canMergeNow || data.hasOverridableBlockers),
		"allOverridableChecksOk":       !data.hasOverridableBlockers,
		"textBypassRules":              ctx.Locale.Tr("repo.pulls.merge_bypass_rules"),
		"pullHeadCommitID":             prInfo.CompareInfo.HeadCommitID,
		"defaultMergeStyle":            mergeStyle,
		"mergeStyles":                  mergeStyles,
		"mergeMessageFieldPlaceHolder": ctx.Locale.Tr("repo.editor.commit_message_desc"),
	}
}
