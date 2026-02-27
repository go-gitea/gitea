// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
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

// mergeFormParams holds values computed in preparePullViewReviewAndMerge, passed
// directly to avoid re-reading them from ctx.Data.
type mergeFormParams struct {
	AllowMerge                        bool
	ProtectedBranch                   *git_model.ProtectedBranch
	PullRequestsConfig                *repo_model.PullRequestsConfig
	MergeStyle                        repo_model.MergeStyle
	DefaultMergeMessage               string
	DefaultMergeBody                  string
	DefaultSquashMergeMessage         string
	DefaultSquashMergeBody            string
	HasPendingPullRequestMerge        bool
	PendingPullRequestMerge           *pull_model.AutoMerge
	IsBlockedByApprovals              bool
	IsBlockedByRejection              bool
	IsBlockedByOfficialReviewRequests bool
	IsBlockedByOutdatedBranch         bool
	IsBlockedByChangedProtectedFiles  bool
}

// preparePullViewMergeFormData builds the JSON data for the merge form Vue component.
// It must be called after preparePullViewReviewAndMerge so all necessary ctx.Data values exist.
func preparePullViewMergeFormData(ctx *context.Context, issue *issues_model.Issue, params *mergeFormParams) {
	pull := issue.PullRequest

	if pull.HasMerged || issue.IsClosed {
		return
	}

	if !pull.CanAutoMerge() && !pull.IsEmpty() {
		return
	}

	if !params.AllowMerge {
		return
	}

	prConfig := params.PullRequestsConfig
	if !(prConfig.AllowMerge || prConfig.AllowRebase || prConfig.AllowRebaseMerge || prConfig.AllowSquash || prConfig.AllowFastForwardOnly) {
		return
	}

	// Values set by preparePullViewPullInfo
	enableStatusCheck, _ := ctx.Data["EnableStatusCheck"].(bool)
	requiredStatusCheckSuccess := false
	if statusCheckData, ok := ctx.Data["StatusCheckData"].(*pullCommitStatusCheckData); ok && statusCheckData != nil {
		requiredStatusCheckSuccess = statusCheckData.RequiredChecksState.IsSuccess()
	}

	notAllOverridableChecksOk := params.IsBlockedByApprovals || params.IsBlockedByRejection ||
		params.IsBlockedByOfficialReviewRequests || params.IsBlockedByOutdatedBranch ||
		params.IsBlockedByChangedProtectedFiles || (enableStatusCheck && !requiredStatusCheckSuccess)

	isRepoAdmin := ctx.IsSigned && (ctx.Repo.IsAdmin() || ctx.Doer.IsAdmin)
	willSign, _ := ctx.Data["WillSign"].(bool) // set by preparePullViewSigning

	var requireSigned, blockAdminMergeOverride bool
	if params.ProtectedBranch != nil {
		requireSigned = params.ProtectedBranch.RequireSignedCommits
		blockAdminMergeOverride = params.ProtectedBranch.BlockAdminMergeOverride
	}

	canMergeNow := ((!blockAdminMergeOverride && isRepoAdmin) || !notAllOverridableChecksOk) &&
		(!params.AllowMerge || !requireSigned || willSign)

	generalHideAutoMerge := canMergeNow && !notAllOverridableChecksOk

	hasPendingPullRequestMergeTip := ""
	if params.HasPendingPullRequestMerge && params.PendingPullRequestMerge != nil {
		createdPRMergeStr := templates.TimeSince(params.PendingPullRequestMerge.CreatedUnix)
		hasPendingPullRequestMergeTip = ctx.Locale.TrString("repo.pulls.auto_merge_has_pending_schedule", params.PendingPullRequestMerge.Doer.Name, createdPRMergeStr)
	}

	// Values set by preparePullViewPullInfo and preparePullViewDeleteBranch
	pullHeadCommitID, _ := ctx.Data["PullHeadCommitID"].(string)
	isPullBranchDeletable, _ := ctx.Data["IsPullBranchDeletable"].(bool)
	headTarget, _ := ctx.Data["HeadTarget"].(string)
	getCommitMessages, _ := ctx.Data["GetCommitMessages"].(string)

	form := &mergeFormField{
		BaseLink:                       issue.Link(),
		TextCancel:                     ctx.Locale.TrString("cancel"),
		TextDeleteBranch:               ctx.Locale.TrString("repo.branch.delete", headTarget),
		TextAutoMergeButtonWhenSucceed: ctx.Locale.TrString("repo.pulls.auto_merge_button_when_succeed"),
		TextAutoMergeWhenSucceed:       ctx.Locale.TrString("repo.pulls.auto_merge_when_succeed"),
		TextAutoMergeCancelSchedule:    ctx.Locale.TrString("repo.pulls.auto_merge_cancel_schedule"),
		TextClearMergeMessage:          ctx.Locale.TrString("repo.pulls.clear_merge_message"),
		TextClearMergeMessageHint:      ctx.Locale.TrString("repo.pulls.clear_merge_message_hint"),
		TextMergeCommitID:              ctx.Locale.TrString("repo.pulls.merge_commit_id"),
		CanMergeNow:                    canMergeNow,
		AllOverridableChecksOk:         !notAllOverridableChecksOk,
		EmptyCommit:                    pull.IsEmpty(),
		PullHeadCommitID:               pullHeadCommitID,
		IsPullBranchDeletable:          isPullBranchDeletable,
		DefaultMergeStyle:              string(params.MergeStyle),
		DefaultDeleteBranchAfterMerge:  prConfig.DefaultDeleteBranchAfterMerge,
		MergeMessageFieldPlaceHolder:   ctx.Locale.TrString("repo.editor.commit_message_desc"),
		DefaultMergeMessage:            params.DefaultMergeBody,
		HasPendingPullRequestMerge:     params.HasPendingPullRequestMerge,
		HasPendingPullRequestMergeTip:  hasPendingPullRequestMergeTip,
		MergeStyles: []mergeStyleField{
			{
				Name:                  "merge",
				Allowed:               prConfig.AllowMerge,
				TextDoMerge:           ctx.Locale.TrString("repo.pulls.merge_pull_request"),
				MergeTitleFieldText:   params.DefaultMergeMessage,
				MergeMessageFieldText: params.DefaultMergeBody,
				HideAutoMerge:         generalHideAutoMerge,
			},
			{
				Name:                  "rebase",
				Allowed:               prConfig.AllowRebase,
				TextDoMerge:           ctx.Locale.TrString("repo.pulls.rebase_merge_pull_request"),
				HideMergeMessageTexts: true,
				HideAutoMerge:         generalHideAutoMerge,
			},
			{
				Name:                  "rebase-merge",
				Allowed:               prConfig.AllowRebaseMerge,
				TextDoMerge:           ctx.Locale.TrString("repo.pulls.rebase_merge_commit_pull_request"),
				MergeTitleFieldText:   params.DefaultMergeMessage,
				MergeMessageFieldText: params.DefaultMergeBody,
				HideAutoMerge:         generalHideAutoMerge,
			},
			{
				Name:                  "squash",
				Allowed:               prConfig.AllowSquash,
				TextDoMerge:           ctx.Locale.TrString("repo.pulls.squash_merge_pull_request"),
				MergeTitleFieldText:   params.DefaultSquashMergeMessage,
				MergeMessageFieldText: getCommitMessages + params.DefaultSquashMergeBody,
				HideAutoMerge:         generalHideAutoMerge,
			},
			{
				Name:                  "fast-forward-only",
				Allowed:               prConfig.AllowFastForwardOnly && pull.CommitsBehind == 0,
				TextDoMerge:           ctx.Locale.TrString("repo.pulls.fast_forward_only_merge_pull_request"),
				HideMergeMessageTexts: true,
				HideAutoMerge:         generalHideAutoMerge,
			},
			{
				Name:                  "manually-merged",
				Allowed:               prConfig.AllowManualMerge,
				TextDoMerge:           ctx.Locale.TrString("repo.pulls.merge_manually"),
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
