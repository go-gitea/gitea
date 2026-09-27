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

// MockPullMergeBoxes returns the template data of merge boxes in various states for the devtest page
func MockPullMergeBoxes(ctx *context.Context) (scenarios []map[string]any) {
	repo := &repo_model.Repository{ID: 1, OwnerName: "user2", Name: "repo1"}
	prConfig := &repo_model.PullRequestsConfig{AllowMerge: true, AllowRebase: true, AllowRebaseMerge: true, AllowSquash: true, AllowFastForwardOnly: true, AllowManualMerge: true}
	checks := func(required string, statuses ...*git_model.CommitStatus) *pullCommitStatusCheckData {
		actionsStatuses := actions_module.CommitActionsStatusMap{}
		for i, cs := range statuses {
			cs.ID, cs.TargetURL = int64(i+1), "#"
			if cs.Description == "Running" {
				actionsStatuses[cs.ID] = actions_model.StatusRunning
			}
		}
		return &pullCommitStatusCheckData{
			IsContextRequired: func(context string) bool { return context == required },
			ActionsStatuses:   actionsStatuses,
			Groups:            groupStatusChecks(statuses, actionsStatuses),
		}
	}
	status := func(context string, state commitstatus.CommitStatusState, description string) *git_model.CommitStatus {
		return &git_model.CommitStatus{Context: context, State: state, Description: description}
	}
	passed := checks("ci/test",
		status("ci/lint", commitstatus.CommitStatusSuccess, "Successful in 32s"),
		status("ci/test", commitstatus.CommitStatusSuccess, "Successful in 2m"),
		status("ci/build (linux)", commitstatus.CommitStatusSuccess, "Successful in 4m"),
		status("ci/build (windows)", commitstatus.CommitStatusSuccess, "Successful in 6m"),
		status("ci/build (macos)", commitstatus.CommitStatusSuccess, "Successful in 5m"),
		status("ci/e2e (chromium)", commitstatus.CommitStatusSuccess, "Successful in 3m"),
		status("ci/e2e (firefox)", commitstatus.CommitStatusSuccess, "Successful in 4m"),
		status("security/codeql", commitstatus.CommitStatusSuccess, "No new alerts"),
	)
	add := func(title string, setup func(prInfo *pullRequestViewInfo)) {
		pull := &issues_model.PullRequest{Index: 1, Status: issues_model.PullRequestStatusMergeable, HeadRepo: repo, BaseRepo: repo, HeadBranch: "feature", BaseBranch: "main"}
		issue := &issues_model.Issue{Index: 1, IsPull: true, Title: "Add feature", Repo: repo, PullRequest: pull}
		pull.Issue = issue
		data := &pullMergeBoxData{ShowMergeBox: true, hasPermToMerge: true, canMergeNow: true, IsPullBranchDeletable: true}
		prInfo := &pullRequestViewInfo{issue: issue, MergeBoxData: data, headTarget: "user2:feature"}
		setup(prInfo)
		if issue.IsClosed {
			prInfo.prepareMergeBoxClosedSection(ctx)
		} else {
			prInfo.prepareMergeBoxSections(ctx)
			prInfo.prepareMergeBoxFormProps(ctx, prConfig)
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

	add("Ready to merge", func(prInfo *pullRequestViewInfo) {
		prInfo.MergeBoxData.StatusCheckData = passed
	})
	add("Review required, checks in progress", func(prInfo *pullRequestViewInfo) {
		data := prInfo.MergeBoxData
		line := ctx.Locale.TrN(1, "repo.pulls.approvals_required_1", "repo.pulls.approvals_required_n", 1)
		data.ReviewSection = &pullMergeBoxSection{Icon: "octicon-x", IconClass: "red", Title: ctx.Locale.Tr("repo.pulls.review_required"), Details: []template.HTML{line}}
		data.addOverridableBlocker(line)
		data.StatusCheckData = checks("ci/test",
			status("ci/lint", commitstatus.CommitStatusSuccess, "Successful in 32s"),
			status("ci/test", commitstatus.CommitStatusPending, "Running"),
			status("ci/build (linux)", commitstatus.CommitStatusPending, "Running"),
			status("ci/build (macos)", commitstatus.CommitStatusPending, "Running"),
			status("ci/build (windows)", commitstatus.CommitStatusPending, "Queued"),
			status("ci/e2e (chromium)", commitstatus.CommitStatusPending, "Waiting for ci/build"),
			status("ci/e2e (firefox)", commitstatus.CommitStatusPending, "Waiting for ci/build"),
			status("ci/docs", commitstatus.CommitStatusSuccess, "Successful in 12s"),
			status("security/codeql", commitstatus.CommitStatusSuccess, "No new alerts"),
		)
		data.addOverridableBlocker(ctx.Locale.Tr("repo.pulls.required_status_check_missing"))
	})
	add("Some checks were not successful", func(prInfo *pullRequestViewInfo) {
		data := prInfo.MergeBoxData
		data.StatusCheckData = checks("ci/lint",
			status("ci/lint", commitstatus.CommitStatusFailure, "Failing after 32s"),
			status("ci/audit", commitstatus.CommitStatusWarning, "2 advisories"),
			status("ci/test (sqlite)", commitstatus.CommitStatusError, "Failing after 3m"),
			status("ci/build (linux)", commitstatus.CommitStatusPending, "Queued"),
			status("ci/build (windows)", commitstatus.CommitStatusPending, "Queued"),
			status("ci/e2e (chromium)", commitstatus.CommitStatusPending, "Running"),
			status("ci/e2e (firefox)", commitstatus.CommitStatusPending, "Running"),
			status("ci/docs", commitstatus.CommitStatusSkipped, "Skipped"),
			status("ci/release", commitstatus.CommitStatusSkipped, "Skipped"),
			status("ci/deploy", commitstatus.CommitStatusSkipped, "Skipped"),
			status("ci/test (mysql)", commitstatus.CommitStatusSuccess, "Successful in 5m"),
			status("ci/test (pgsql)", commitstatus.CommitStatusSuccess, "Successful in 5m"),
			status("security/codeql", commitstatus.CommitStatusSuccess, "No new alerts"),
		)
		data.addOverridableBlocker(ctx.Locale.Tr("repo.pulls.required_status_check_failed"))
	})
	add("All checks have failed", func(prInfo *pullRequestViewInfo) {
		prInfo.MergeBoxData.StatusCheckData = checks("ci/test",
			status("ci/lint", commitstatus.CommitStatusFailure, "Failing after 32s"),
			status("ci/test", commitstatus.CommitStatusError, "Failing after 2m"),
			status("ci/build (linux)", commitstatus.CommitStatusFailure, "Failing after 1m"),
			status("ci/build (windows)", commitstatus.CommitStatusFailure, "Failing after 3m"),
		)
	})
	add("Changes approved, out of date", func(prInfo *pullRequestViewInfo) {
		data := prInfo.MergeBoxData
		data.ReviewSection = &pullMergeBoxSection{
			Icon: "octicon-check", IconClass: "green", Title: ctx.Locale.Tr("repo.pulls.changes_approved"),
			Details: []template.HTML{ctx.Locale.TrN(2, "repo.pulls.approvals_granted_1", "repo.pulls.approvals_granted_n", 2)},
		}
		data.StatusCheckData = passed
		data.ShowUpdatePullInfo = true
		data.UpdateStyleOptions = []*pullUpdateAction{
			{URL: "#", Text: ctx.Tr("repo.pulls.update_branch"), Selected: true, Description: ctx.Tr("repo.pulls.update_branch_desc"), ButtonText: ctx.Tr("repo.pulls.update_branch_button")},
			{URL: "#", Text: ctx.Tr("repo.pulls.update_branch_rebase"), Description: ctx.Tr("repo.pulls.update_branch_rebase_desc"), ButtonText: ctx.Tr("repo.pulls.update_branch_rebase_button")},
		}
		data.UpdatePrimaryAction = data.UpdateStyleOptions[0]
	})
	add("Changes requested, auto-merge enabled", func(prInfo *pullRequestViewInfo) {
		data := prInfo.MergeBoxData
		line := ctx.Locale.Tr("repo.pulls.blocked_by_rejection")
		data.ReviewSection = &pullMergeBoxSection{Icon: "octicon-file-diff", IconClass: "red", Title: ctx.Locale.Tr("repo.pulls.changes_requested"), Details: []template.HTML{line}}
		data.addOverridableBlocker(line)
		data.AutoMerge = &pull_model.AutoMerge{Doer: &user_model.User{Name: "user2"}, MergeStyle: repo_model.MergeStyleSquash}
		data.CanCancelAutoMerge = true
	})
	add("Conflicts", func(prInfo *pullRequestViewInfo) {
		pull := prInfo.issue.PullRequest
		pull.Status, pull.ConflictedFiles = issues_model.PullRequestStatusConflict, []string{"README.md", "main.go"}
	})
	add("Checking for mergeability", func(prInfo *pullRequestViewInfo) {
		prInfo.issue.PullRequest.Status = issues_model.PullRequestStatusChecking
	})
	add("Work in progress", func(prInfo *pullRequestViewInfo) {
		prInfo.workInProgressPrefix, prInfo.issue.Title = "WIP:", "WIP: Add feature"
		prInfo.MergeBoxData.isMergeBlocked = true
	})
	add("Merged", func(prInfo *pullRequestViewInfo) {
		prInfo.issue.IsClosed, prInfo.issue.PullRequest.HasMerged = true, true
	})
	add("Closed with unmerged commits", func(prInfo *pullRequestViewInfo) {
		prInfo.issue.IsClosed = true
	})
	return scenarios
}
