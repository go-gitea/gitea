// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"html/template"

	actions_model "gitea.dev/models/actions"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/services/context"
)

func MockPullMergeBoxes(ctx *context.Context) (scenarios []map[string]any) {
	repo := &repo_model.Repository{ID: 1, OwnerName: "user2", Name: "repo1"}
	prConfig := &repo_model.PullRequestsConfig{AllowMerge: true, AllowRebase: true, AllowRebaseMerge: true, AllowSquash: true, AllowFastForwardOnly: true, AllowManualMerge: true}
	checks := func(statuses ...*git_model.CommitStatus) *pullCommitStatusCheckData {
		actionsStatuses := actions_module.CommitActionsStatusMap{}
		for i, cs := range statuses {
			cs.ID, cs.TargetURL = int64(i+1), "#"
			if cs.Description == "Running" {
				actionsStatuses[cs.ID] = actions_model.StatusRunning
			}
		}
		data := &pullCommitStatusCheckData{IsContextRequired: func(context string) bool { return context == "ci/test" }, ActionsStatuses: actionsStatuses}
		data.groupStatuses(statuses)
		return data
	}
	status := func(context string, state commitstatus.CommitStatusState, description string) *git_model.CommitStatus {
		return &git_model.CommitStatus{Context: context, State: state, Description: description}
	}
	review := func(data *pullMergeBoxData, icon, iconClass, titleKey string, detail template.HTML, blocking bool) {
		data.ReviewSection = &pullMergeBoxSection{Icon: icon, IconClass: iconClass, Title: ctx.Locale.Tr(titleKey), Details: []template.HTML{detail}}
		if blocking {
			data.addOverridableBlocker(detail)
		}
	}
	add := func(title string, setup func(prInfo *pullRequestViewInfo)) {
		pull := &issues_model.PullRequest{Index: 1, Status: issues_model.PullRequestStatusMergeable, HeadRepo: repo, BaseRepo: repo, HeadBranch: "feature", BaseBranch: "main"}
		issue := &issues_model.Issue{Index: 1, IsPull: true, Title: "Add feature", Repo: repo, PullRequest: pull}
		pull.Issue = issue
		data := &pullMergeBoxData{hasPermToMerge: true, canBypassProtection: true}
		prInfo := &pullRequestViewInfo{issue: issue, MergeBoxData: data, headTarget: "user2:feature"}
		setup(prInfo)
		if issue.IsClosed {
			data.IsPullBranchDeletable = true
			prInfo.prepareMergeBoxClosedSection(ctx)
		} else {
			prInfo.prepareMergeBoxSections(ctx)
			prInfo.prepareMergeBoxFormProps(ctx, prConfig, nil)
		}
		scenarios = append(scenarios, map[string]any{
			"Title":                           title,
			"PullMergeBoxData":                data,
			"Issue":                           issue,
			"DeleteBranchLink":                "#",
			"HasIssuesOrPullsWritePermission": true,
			"WorkInProgressPrefix":            prInfo.workInProgressPrefix,
		})
	}

	add("Ready to merge, out of date", func(prInfo *pullRequestViewInfo) {
		data := prInfo.MergeBoxData
		review(data, "octicon-check", "tw-bg-green", "repo.pulls.changes_approved", ctx.Locale.TrN(2, "repo.pulls.approvals_granted_1", "repo.pulls.approvals_granted_n", 2), false)
		data.StatusCheckData = checks(status("ci/lint", commitstatus.CommitStatusSuccess, "Successful in 32s"), status("ci/test", commitstatus.CommitStatusSuccess, "Successful in 2m"))
		data.ShowUpdatePullInfo = true
		data.UpdateStyleOptions = []*pullUpdateAction{
			{URL: "#", Text: ctx.Tr("repo.pulls.update_branch"), Selected: true, Description: ctx.Tr("repo.pulls.update_branch_desc"), ButtonText: ctx.Tr("repo.pulls.update_branch_button")},
			{URL: "#", Text: ctx.Tr("repo.pulls.update_branch_rebase"), Description: ctx.Tr("repo.pulls.update_branch_rebase_desc"), ButtonText: ctx.Tr("repo.pulls.update_branch_rebase_button")},
		}
		data.UpdatePrimaryAction = data.UpdateStyleOptions[0]
	})
	add("Review required, checks in progress", func(prInfo *pullRequestViewInfo) {
		review(prInfo.MergeBoxData, "octicon-x", "tw-bg-red", "repo.pulls.review_required", ctx.Locale.TrN(1, "repo.pulls.approvals_required_1", "repo.pulls.approvals_required_n", 1), true)
		prInfo.MergeBoxData.StatusCheckData = checks(
			status("ci/lint", commitstatus.CommitStatusSuccess, "Successful in 32s"),
			status("ci/test", commitstatus.CommitStatusPending, "Running"),
			status("ci/build", commitstatus.CommitStatusPending, "Queued"),
		)
	})
	add("Some checks were not successful", func(prInfo *pullRequestViewInfo) {
		prInfo.MergeBoxData.StatusCheckData = checks(
			status("ci/test", commitstatus.CommitStatusFailure, "Failing after 32s"),
			status("ci/audit", commitstatus.CommitStatusWarning, "2 advisories"),
			status("ci/e2e", commitstatus.CommitStatusPending, "Running"),
			status("ci/docs", commitstatus.CommitStatusSkipped, "Skipped"),
			status("ci/lint", commitstatus.CommitStatusSuccess, "Successful in 5m"),
		)
		prInfo.MergeBoxData.hasOverridableBlockers = true
	})
	add("Changes requested, auto-merge enabled", func(prInfo *pullRequestViewInfo) {
		data := prInfo.MergeBoxData
		review(data, "octicon-file-diff", "tw-bg-red", "repo.pulls.changes_requested", ctx.Locale.Tr("repo.pulls.blocked_by_rejection"), true)
		data.AutoMerge = &pull_model.AutoMerge{Doer: &user_model.User{Name: "user2"}, MergeStyle: repo_model.MergeStyleSquash}
		data.CanCancelAutoMerge = true
	})
	add("Work in progress, conflicts", func(prInfo *pullRequestViewInfo) {
		prInfo.workInProgressPrefix, prInfo.issue.Title = "WIP:", "WIP: Add feature"
		prInfo.issue.PullRequest.Status, prInfo.issue.PullRequest.ConflictedFiles = issues_model.PullRequestStatusConflict, []string{"README.md", "main.go"}
	})
	add("Merged", func(prInfo *pullRequestViewInfo) {
		prInfo.issue.IsClosed, prInfo.issue.PullRequest.HasMerged = true, true
	})
	return scenarios
}
