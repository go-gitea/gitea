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

// pullViewMergeInputs carries data from preparePullViewPullInfo to the merge form builder,
// avoiding ctx.Data round-trips for values set by other prepare functions.
type pullViewMergeInputs struct {
	PullHeadCommitID  string
	HeadTarget        string
	GetCommitMessages string
	StatusCheckData   *pullCommitStatusCheckData
}

type mergeFormParams struct {
	AllowMerge                        bool
	ProtectedBranch                   *git_model.ProtectedBranch
	PrConfig                          *repo_model.PullRequestsConfig
	MergeStyle                        repo_model.MergeStyle
	DefaultMergeMessage               string
	DefaultMergeBody                  string
	DefaultSquashMergeMessage         string
	DefaultSquashMergeBody            string
	GetCommitMessages                 string
	PendingPullRequestMerge           *pull_model.AutoMerge
	IsBlockedByApprovals              bool
	IsBlockedByRejection              bool
	IsBlockedByOfficialReviewRequests bool
	WillSign                          bool
	IsPullBranchDeletable             bool
	PullHeadCommitID                  string
	HeadTarget                        string
	StatusCheckData                   *pullCommitStatusCheckData
}

// preparePullViewMergeFormData builds the JSON data for the merge form Vue component.
func preparePullViewMergeFormData(ctx *context.Context, issue *issues_model.Issue, params *mergeFormParams) {
	pull := issue.PullRequest

	if pull.HasMerged || issue.IsClosed || (!pull.CanAutoMerge() && !pull.IsEmpty()) || !params.AllowMerge {
		return
	}

	prConfig := params.PrConfig
	if !(prConfig.AllowMerge || prConfig.AllowRebase || prConfig.AllowRebaseMerge || prConfig.AllowSquash || prConfig.AllowFastForwardOnly) {
		return
	}

	pb := params.ProtectedBranch
	requiredStatusCheckSuccess := params.StatusCheckData != nil && params.StatusCheckData.RequiredChecksState.IsSuccess()

	allOverridableChecksOk := !params.IsBlockedByApprovals && !params.IsBlockedByRejection &&
		!params.IsBlockedByOfficialReviewRequests &&
		(pb == nil || !pb.BlockOnOutdatedBranch || pull.CommitsBehind == 0) &&
		len(pull.ChangedProtectedFiles) == 0 &&
		(pb == nil || !pb.EnableStatusCheck || requiredStatusCheckSuccess)

	isRepoAdmin := ctx.IsSigned && (ctx.Repo.IsAdmin() || ctx.Doer.IsAdmin)
	canMergeNow := (((pb == nil || !pb.BlockAdminMergeOverride) && isRepoAdmin) || allOverridableChecksOk) &&
		(pb == nil || !pb.RequireSignedCommits || params.WillSign)
	hideAutoMerge := canMergeNow && allOverridableChecksOk

	hasPendingPullRequestMergeTip := ""
	if params.PendingPullRequestMerge != nil {
		hasPendingPullRequestMergeTip = ctx.Locale.TrString("repo.pulls.auto_merge_has_pending_schedule",
			params.PendingPullRequestMerge.Doer.Name, templates.TimeSince(params.PendingPullRequestMerge.CreatedUnix))
	}

	form := &mergeFormField{
		BaseLink:                       issue.Link(),
		TextCancel:                     ctx.Locale.TrString("cancel"),
		TextDeleteBranch:               ctx.Locale.TrString("repo.branch.delete", params.HeadTarget),
		TextAutoMergeButtonWhenSucceed: ctx.Locale.TrString("repo.pulls.auto_merge_button_when_succeed"),
		TextAutoMergeWhenSucceed:       ctx.Locale.TrString("repo.pulls.auto_merge_when_succeed"),
		TextAutoMergeCancelSchedule:    ctx.Locale.TrString("repo.pulls.auto_merge_cancel_schedule"),
		TextClearMergeMessage:          ctx.Locale.TrString("repo.pulls.clear_merge_message"),
		TextClearMergeMessageHint:      ctx.Locale.TrString("repo.pulls.clear_merge_message_hint"),
		TextMergeCommitID:              ctx.Locale.TrString("repo.pulls.merge_commit_id"),
		CanMergeNow:                    canMergeNow,
		AllOverridableChecksOk:         allOverridableChecksOk,
		EmptyCommit:                    pull.IsEmpty(),
		PullHeadCommitID:               params.PullHeadCommitID,
		IsPullBranchDeletable:          params.IsPullBranchDeletable,
		DefaultMergeStyle:              string(params.MergeStyle),
		DefaultDeleteBranchAfterMerge:  prConfig.DefaultDeleteBranchAfterMerge,
		MergeMessageFieldPlaceHolder:   ctx.Locale.TrString("repo.editor.commit_message_desc"),
		DefaultMergeMessage:            params.DefaultMergeBody,
		HasPendingPullRequestMerge:     params.PendingPullRequestMerge != nil,
		HasPendingPullRequestMergeTip:  hasPendingPullRequestMergeTip,
		MergeStyles: []mergeStyleField{
			{
				Name:                  "merge",
				Allowed:               prConfig.AllowMerge,
				TextDoMerge:           ctx.Locale.TrString("repo.pulls.merge_pull_request"),
				MergeTitleFieldText:   params.DefaultMergeMessage,
				MergeMessageFieldText: params.DefaultMergeBody,
				HideAutoMerge:         hideAutoMerge,
			},
			{
				Name:                  "rebase",
				Allowed:               prConfig.AllowRebase,
				TextDoMerge:           ctx.Locale.TrString("repo.pulls.rebase_merge_pull_request"),
				HideMergeMessageTexts: true,
				HideAutoMerge:         hideAutoMerge,
			},
			{
				Name:                  "rebase-merge",
				Allowed:               prConfig.AllowRebaseMerge,
				TextDoMerge:           ctx.Locale.TrString("repo.pulls.rebase_merge_commit_pull_request"),
				MergeTitleFieldText:   params.DefaultMergeMessage,
				MergeMessageFieldText: params.DefaultMergeBody,
				HideAutoMerge:         hideAutoMerge,
			},
			{
				Name:                  "squash",
				Allowed:               prConfig.AllowSquash,
				TextDoMerge:           ctx.Locale.TrString("repo.pulls.squash_merge_pull_request"),
				MergeTitleFieldText:   params.DefaultSquashMergeMessage,
				MergeMessageFieldText: params.GetCommitMessages + params.DefaultSquashMergeBody,
				HideAutoMerge:         hideAutoMerge,
			},
			{
				Name:                  "fast-forward-only",
				Allowed:               prConfig.AllowFastForwardOnly && pull.CommitsBehind == 0,
				TextDoMerge:           ctx.Locale.TrString("repo.pulls.fast_forward_only_merge_pull_request"),
				HideMergeMessageTexts: true,
				HideAutoMerge:         hideAutoMerge,
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
}
