// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"strings"

	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
	notify_service "gitea.dev/services/notify"
)

func init() {
	notify_service.RegisterNotifier(&mergeRefNotifier{})
}

type mergeRefNotifier struct {
	notify_service.NullNotifier
}

func (*mergeRefNotifier) IssueChangeStatus(ctx context.Context, _doer *user_model.User, _commitID string, issue *issues_model.Issue, _comment *issues_model.Comment, isClosed bool) {
	if !issue.IsPull {
		return
	}
	if err := issue.LoadPullRequest(ctx); err != nil {
		log.Error("LoadPullRequest: %v", err)
	} else if !isClosed {
		StartPullRequestCheckImmediately(ctx, issue.PullRequest)
	} else if err := git.RemoveRef(ctx, issue.Repo, git.MergeRefNameFromPullIndex(issue.Index).String()); err != nil {
		log.Error("RemoveRef: %v", err)
	}
}

// UpdateMergeRef refreshes refs/pull/<index>/merge for headCommitID and returns its commit, empty when closed, conflicting or git lacks merge-tree
func UpdateMergeRef(ctx context.Context, pr *issues_model.PullRequest, headCommitID string) (string, error) {
	if !git.DefaultFeatures().SupportGitMergeTree || pr.Issue.IsClosed {
		return "", nil
	}
	baseCommitID, err := git.GetFullCommitID(ctx, pr.BaseRepo, git.BranchPrefix+pr.BaseBranch)
	if err != nil {
		return "", err
	}
	treeID, conflicted, _, err := git.MergeTree(ctx, pr.BaseRepo, baseCommitID, headCommitID, "")
	if err != nil || conflicted {
		return "", err
	}
	return updateMergeRef(ctx, pr, baseCommitID, headCommitID, treeID)
}

func updateMergeRef(ctx context.Context, pr *issues_model.PullRequest, baseCommitID, headCommitID, treeID string) (string, error) {
	stdout, _, err := gitcmd.NewCommand("commit-tree", "--no-gpg-sign", "-p").AddDynamicArguments(baseCommitID).
		AddArguments("-p").AddDynamicArguments(headCommitID).
		AddOptionValues("-m", "Merge "+headCommitID+" into "+baseCommitID).
		AddDynamicArguments(treeID).WithRepo(pr.BaseRepo).RunStdString(ctx)
	if err != nil {
		return "", err
	}
	commitID := strings.TrimSpace(stdout)
	return commitID, git.UpdateRef(ctx, pr.BaseRepo, git.MergeRefNameFromPullIndex(pr.Index).String(), commitID)
}
