// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
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
func preparePullViewMergeFormData(ctx *context.Context, issue *issues_model.Issue) {
	pull := issue.PullRequest

	allowMerge, _ := ctx.Data["AllowMerge"].(bool)
	if pull.HasMerged || issue.IsClosed || (!pull.CanAutoMerge() && !pull.IsEmpty()) || !allowMerge {
		return
	}

	prUnit, err := issue.Repo.GetUnit(ctx, unit.TypePullRequests)
	if err != nil {
		ctx.ServerError("GetUnit", err)
		return
	}
	prConfig := prUnit.PullRequestsConfig()
	if !(prConfig.AllowMerge || prConfig.AllowRebase || prConfig.AllowRebaseMerge || prConfig.AllowSquash || prConfig.AllowFastForwardOnly) {
		return
	}

	pb, _ := ctx.Data["ProtectedBranch"].(*git_model.ProtectedBranch)
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

	allOverridableChecksOk := !isBlockedByApprovals && !isBlockedByRejection &&
		!isBlockedByOfficialReviewRequests && !isBlockedByOutdatedBranch &&
		!isBlockedByChangedProtectedFiles && (!enableStatusCheck || requiredStatusCheckSuccess)

	willSign, _ := ctx.Data["WillSign"].(bool)
	isRepoAdmin := ctx.IsSigned && (ctx.Repo.IsAdmin() || ctx.Doer.IsAdmin)
	var requireSigned, blockAdminMergeOverride bool
	if pb != nil {
		requireSigned = pb.RequireSignedCommits
		blockAdminMergeOverride = pb.BlockAdminMergeOverride
	}
	canMergeNow := ((!blockAdminMergeOverride && isRepoAdmin) || allOverridableChecksOk) &&
		(!allowMerge || !requireSigned || willSign)

	hideAutoMerge := canMergeNow && allOverridableChecksOk

	hasPendingPullRequestMerge, _ := ctx.Data["HasPendingPullRequestMerge"].(bool)
	hasPendingPullRequestMergeTip := ""
	if pendingPullRequestMerge, ok := ctx.Data["PendingPullRequestMerge"].(*pull_model.AutoMerge); ok && pendingPullRequestMerge != nil {
		hasPendingPullRequestMergeTip = ctx.Locale.TrString("repo.pulls.auto_merge_has_pending_schedule",
			pendingPullRequestMerge.Doer.Name, templates.TimeSince(pendingPullRequestMerge.CreatedUnix))
	}

	defaultMergeMessage, _ := ctx.Data["DefaultMergeMessage"].(string)
	defaultMergeBody, _ := ctx.Data["DefaultMergeBody"].(string)
	defaultSquashMergeMessage, _ := ctx.Data["DefaultSquashMergeMessage"].(string)
	defaultSquashMergeBody, _ := ctx.Data["DefaultSquashMergeBody"].(string)
	getCommitMessages, _ := ctx.Data["GetCommitMessages"].(string)
	mergeStyle, _ := ctx.Data["MergeStyle"].(repo_model.MergeStyle)
	pullHeadCommitID, _ := ctx.Data["PullHeadCommitID"].(string)
	isPullBranchDeletable, _ := ctx.Data["IsPullBranchDeletable"].(bool)
	headTarget, _ := ctx.Data["HeadTarget"].(string)

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
		AllOverridableChecksOk:         allOverridableChecksOk,
		EmptyCommit:                    pull.IsEmpty(),
		PullHeadCommitID:               pullHeadCommitID,
		IsPullBranchDeletable:          isPullBranchDeletable,
		DefaultMergeStyle:              string(mergeStyle),
		DefaultDeleteBranchAfterMerge:  prConfig.DefaultDeleteBranchAfterMerge,
		MergeMessageFieldPlaceHolder:   ctx.Locale.TrString("repo.editor.commit_message_desc"),
		DefaultMergeMessage:            defaultMergeBody,
		HasPendingPullRequestMerge:     hasPendingPullRequestMerge,
		HasPendingPullRequestMergeTip:  hasPendingPullRequestMergeTip,
		MergeStyles: []mergeStyleField{
			{
				Name:                  "merge",
				Allowed:               prConfig.AllowMerge,
				TextDoMerge:           ctx.Locale.TrString("repo.pulls.merge_pull_request"),
				MergeTitleFieldText:   defaultMergeMessage,
				MergeMessageFieldText: defaultMergeBody,
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
				MergeTitleFieldText:   defaultMergeMessage,
				MergeMessageFieldText: defaultMergeBody,
				HideAutoMerge:         hideAutoMerge,
			},
			{
				Name:                  "squash",
				Allowed:               prConfig.AllowSquash,
				TextDoMerge:           ctx.Locale.TrString("repo.pulls.squash_merge_pull_request"),
				MergeTitleFieldText:   defaultSquashMergeMessage,
				MergeMessageFieldText: getCommitMessages + defaultSquashMergeBody,
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
