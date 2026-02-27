// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"

	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

type mergeStyleField struct {
	Name                  string `json:"name"`
	Allowed               bool   `json:"allowed"`
	TextDoMerge           string `json:"textDoMerge"`
	MergeTitleFieldText   string `json:"mergeTitleFieldText,omitempty"`
	MergeMessageFieldText string `json:"mergeMessageFieldText,omitempty"`
	HideMergeMessageTexts bool   `json:"hideMergeMessageTexts,omitempty"`
	HideAutoMerge         bool   `json:"hideAutoMerge"`
}

type mergeFormField struct {
	BaseLink                       string            `json:"baseLink"`
	TextCancel                     string            `json:"textCancel"`
	TextDeleteBranch               string            `json:"textDeleteBranch"`
	TextAutoMergeButtonWhenSucceed string            `json:"textAutoMergeButtonWhenSucceed"`
	TextAutoMergeWhenSucceed       string            `json:"textAutoMergeWhenSucceed"`
	TextAutoMergeCancelSchedule    string            `json:"textAutoMergeCancelSchedule"`
	TextClearMergeMessage          string            `json:"textClearMergeMessage"`
	TextClearMergeMessageHint      string            `json:"textClearMergeMessageHint"`
	TextMergeCommitID              string            `json:"textMergeCommitId"`
	CanMergeNow                    bool              `json:"canMergeNow"`
	AllOverridableChecksOk         bool              `json:"allOverridableChecksOk"`
	EmptyCommit                    bool              `json:"emptyCommit"`
	PullHeadCommitID               string            `json:"pullHeadCommitID"`
	IsPullBranchDeletable          bool              `json:"isPullBranchDeletable"`
	DefaultMergeStyle              string            `json:"defaultMergeStyle"`
	DefaultDeleteBranchAfterMerge  bool              `json:"defaultDeleteBranchAfterMerge"`
	MergeMessageFieldPlaceHolder   string            `json:"mergeMessageFieldPlaceHolder"`
	DefaultMergeMessage            string            `json:"defaultMergeMessage"`
	HasPendingPullRequestMerge     bool              `json:"hasPendingPullRequestMerge"`
	HasPendingPullRequestMergeTip  string            `json:"hasPendingPullRequestMergeTip"`
	MergeStyles                    []mergeStyleField `json:"mergeStyles"`
}

// preparePullViewMergeFormData builds the JSON data for the merge form Vue component.
// It must be called after preparePullViewReviewAndMerge so all necessary ctx.Data values exist.
func preparePullViewMergeFormData(ctx *context.Context, issue *issues_model.Issue) {
	if !issue.IsPull {
		return
	}
	pull := issue.PullRequest

	if pull.HasMerged || issue.IsClosed {
		return
	}

	// The merge form is only shown when CanAutoMerge or IsEmpty
	if !pull.CanAutoMerge() && !pull.IsEmpty() {
		return
	}

	allowMerge, _ := ctx.Data["AllowMerge"].(bool)
	if !allowMerge {
		return
	}

	prUnit, err := issue.Repo.GetUnit(ctx, unit.TypePullRequests)
	if err != nil {
		return
	}
	prConfig := prUnit.PullRequestsConfig()

	if !(prConfig.AllowMerge || prConfig.AllowRebase || prConfig.AllowRebaseMerge || prConfig.AllowSquash || prConfig.AllowFastForwardOnly) {
		return
	}

	// Compute notAllOverridableChecksOk (same logic as template)
	isBlockedByApprovals, _ := ctx.Data["IsBlockedByApprovals"].(bool)
	isBlockedByRejection, _ := ctx.Data["IsBlockedByRejection"].(bool)
	isBlockedByOfficialReviewRequests, _ := ctx.Data["IsBlockedByOfficialReviewRequests"].(bool)
	isBlockedByOutdatedBranch, _ := ctx.Data["IsBlockedByOutdatedBranch"].(bool)
	isBlockedByChangedProtectedFiles, _ := ctx.Data["IsBlockedByChangedProtectedFiles"].(bool)
	enableStatusCheck, _ := ctx.Data["EnableStatusCheck"].(bool)

	requiredStatusCheckSuccess := false
	if statusCheckData, ok := ctx.Data["StatusCheckData"].(*pullCommitStatusCheckData); ok && statusCheckData != nil {
		requiredStatusCheckSuccess = statusCheckData.RequiredChecksState.IsSuccess()
	}

	notAllOverridableChecksOk := isBlockedByApprovals || isBlockedByRejection ||
		isBlockedByOfficialReviewRequests || isBlockedByOutdatedBranch ||
		isBlockedByChangedProtectedFiles || (enableStatusCheck && !requiredStatusCheckSuccess)

	// Compute canMergeNow (same logic as template)
	isRepoAdmin := ctx.IsSigned && (ctx.Repo.IsAdmin() || ctx.Doer.IsAdmin)
	blockAdminMergeOverride := false
	if pb, ok := ctx.Data["ProtectedBranch"].(*git_model.ProtectedBranch); ok && pb != nil {
		blockAdminMergeOverride = pb.BlockAdminMergeOverride
	}
	requireSigned, _ := ctx.Data["RequireSigned"].(bool)
	willSign, _ := ctx.Data["WillSign"].(bool)

	canMergeNow := ((!blockAdminMergeOverride && isRepoAdmin) || !notAllOverridableChecksOk) &&
		(!allowMerge || !requireSigned || willSign)

	generalHideAutoMerge := canMergeNow && !notAllOverridableChecksOk

	// Build hasPendingPullRequestMergeTip
	hasPendingPullRequestMerge, _ := ctx.Data["HasPendingPullRequestMerge"].(bool)
	hasPendingPullRequestMergeTip := ""
	if hasPendingPullRequestMerge {
		if pendingMerge, ok := ctx.Data["PendingPullRequestMerge"].(*pull_model.AutoMerge); ok && pendingMerge != nil {
			createdPRMergeStr := templates.TimeSince(pendingMerge.CreatedUnix)
			hasPendingPullRequestMergeTip = string(ctx.Locale.Tr("repo.pulls.auto_merge_has_pending_schedule", pendingMerge.Doer.Name, createdPRMergeStr))
		}
	}

	// Get values from ctx.Data
	mergeStyle, _ := ctx.Data["MergeStyle"].(repo_model.MergeStyle)
	defaultMergeMessage, _ := ctx.Data["DefaultMergeMessage"].(string)
	defaultMergeBody, _ := ctx.Data["DefaultMergeBody"].(string)
	defaultSquashMergeMessage, _ := ctx.Data["DefaultSquashMergeMessage"].(string)
	defaultSquashMergeBody, _ := ctx.Data["DefaultSquashMergeBody"].(string)
	pullHeadCommitID, _ := ctx.Data["PullHeadCommitID"].(string)
	isPullBranchDeletable, _ := ctx.Data["IsPullBranchDeletable"].(bool)
	headTarget, _ := ctx.Data["HeadTarget"].(string)
	getCommitMessages, _ := ctx.Data["GetCommitMessages"].(string)

	form := &mergeFormField{
		BaseLink:                       issue.Link(),
		TextCancel:                     string(ctx.Locale.Tr("cancel")),
		TextDeleteBranch:               string(ctx.Locale.Tr("repo.branch.delete", headTarget)),
		TextAutoMergeButtonWhenSucceed: string(ctx.Locale.Tr("repo.pulls.auto_merge_button_when_succeed")),
		TextAutoMergeWhenSucceed:       string(ctx.Locale.Tr("repo.pulls.auto_merge_when_succeed")),
		TextAutoMergeCancelSchedule:    string(ctx.Locale.Tr("repo.pulls.auto_merge_cancel_schedule")),
		TextClearMergeMessage:          string(ctx.Locale.Tr("repo.pulls.clear_merge_message")),
		TextClearMergeMessageHint:      string(ctx.Locale.Tr("repo.pulls.clear_merge_message_hint")),
		TextMergeCommitID:              string(ctx.Locale.Tr("repo.pulls.merge_commit_id")),
		CanMergeNow:                    canMergeNow,
		AllOverridableChecksOk:         !notAllOverridableChecksOk,
		EmptyCommit:                    pull.IsEmpty(),
		PullHeadCommitID:               pullHeadCommitID,
		IsPullBranchDeletable:          isPullBranchDeletable,
		DefaultMergeStyle:              string(mergeStyle),
		DefaultDeleteBranchAfterMerge:  prConfig.DefaultDeleteBranchAfterMerge,
		MergeMessageFieldPlaceHolder:   string(ctx.Locale.Tr("repo.editor.commit_message_desc")),
		DefaultMergeMessage:            defaultMergeBody,
		HasPendingPullRequestMerge:     hasPendingPullRequestMerge,
		HasPendingPullRequestMergeTip:  hasPendingPullRequestMergeTip,
		MergeStyles: []mergeStyleField{
			{
				Name:                  "merge",
				Allowed:               prConfig.AllowMerge,
				TextDoMerge:           string(ctx.Locale.Tr("repo.pulls.merge_pull_request")),
				MergeTitleFieldText:   defaultMergeMessage,
				MergeMessageFieldText: defaultMergeBody,
				HideAutoMerge:         generalHideAutoMerge,
			},
			{
				Name:                  "rebase",
				Allowed:               prConfig.AllowRebase,
				TextDoMerge:           string(ctx.Locale.Tr("repo.pulls.rebase_merge_pull_request")),
				HideMergeMessageTexts: true,
				HideAutoMerge:         generalHideAutoMerge,
			},
			{
				Name:                  "rebase-merge",
				Allowed:               prConfig.AllowRebaseMerge,
				TextDoMerge:           string(ctx.Locale.Tr("repo.pulls.rebase_merge_commit_pull_request")),
				MergeTitleFieldText:   defaultMergeMessage,
				MergeMessageFieldText: defaultMergeBody,
				HideAutoMerge:         generalHideAutoMerge,
			},
			{
				Name:                  "squash",
				Allowed:               prConfig.AllowSquash,
				TextDoMerge:           string(ctx.Locale.Tr("repo.pulls.squash_merge_pull_request")),
				MergeTitleFieldText:   defaultSquashMergeMessage,
				MergeMessageFieldText: fmt.Sprintf("%s%s", getCommitMessages, defaultSquashMergeBody),
				HideAutoMerge:         generalHideAutoMerge,
			},
			{
				Name:                  "fast-forward-only",
				Allowed:               prConfig.AllowFastForwardOnly && pull.CommitsBehind == 0,
				TextDoMerge:           string(ctx.Locale.Tr("repo.pulls.fast_forward_only_merge_pull_request")),
				HideMergeMessageTexts: true,
				HideAutoMerge:         generalHideAutoMerge,
			},
			{
				Name:                  "manually-merged",
				Allowed:               prConfig.AllowManualMerge,
				TextDoMerge:           string(ctx.Locale.Tr("repo.pulls.merge_manually")),
				HideMergeMessageTexts: true,
				HideAutoMerge:         true,
			},
		},
	}

	jsonBytes, err := json.Marshal(form)
	if err != nil {
		ctx.ServerError("json.Marshal", err)
		return
	}
	ctx.Data["MergeFormJSON"] = string(jsonBytes)
	ctx.Data["ShowGeneralMergeForm"] = true
}
