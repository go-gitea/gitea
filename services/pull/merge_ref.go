// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"
	"strings"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
)

// UpdateMergeRef refreshes refs/pull/<index>/merge for headCommitID and returns its commit, empty when the merge conflicts or git lacks merge-tree
func UpdateMergeRef(ctx context.Context, pr *issues_model.PullRequest, headCommitID string) (string, error) {
	if !git.DefaultFeatures().SupportGitMergeTree {
		return "", nil
	}
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return "", err
	}
	baseCommitID, err := git.GetFullCommitID(ctx, pr.BaseRepo, git.BranchPrefix+pr.BaseBranch)
	if err != nil {
		return "", err
	}
	treeID, conflicted, _, err := git.MergeTree(ctx, pr.BaseRepo, baseCommitID, headCommitID, "")
	if err != nil {
		return "", err
	}
	return updateMergeRef(ctx, pr, baseCommitID, headCommitID, treeID, conflicted)
}

func updateMergeRef(ctx context.Context, pr *issues_model.PullRequest, baseCommitID, headCommitID, treeID string, conflicted bool) (string, error) {
	mergeRef := git.MergeRefNameFromPullIndex(pr.Index).String()
	if conflicted {
		return "", git.RemoveRef(ctx, pr.BaseRepo, mergeRef)
	}
	stdout, _, err := gitcmd.NewCommand("commit-tree", "--no-gpg-sign", "-p").AddDynamicArguments(baseCommitID).
		AddArguments("-p").AddDynamicArguments(headCommitID).
		AddOptionValues("-m", fmt.Sprintf("Merge %s into %s", headCommitID, baseCommitID)).
		AddDynamicArguments(treeID).WithRepo(pr.BaseRepo).RunStdString(ctx)
	if err != nil {
		return "", err
	}
	commitID := strings.TrimSpace(stdout)
	return commitID, git.UpdateRef(ctx, pr.BaseRepo, mergeRef, commitID)
}
