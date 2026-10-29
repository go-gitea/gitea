// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"

	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
)

// UpdateMergeRef points refs/pull/<index>/merge at a test merge of the pull request head into its base branch, like GitHub does.
// It returns an empty commit ID when the merge has conflicts or git is too old to merge without a worktree.
func UpdateMergeRef(ctx context.Context, pr *issues_model.PullRequest) (string, error) {
	if !git.DefaultFeatures().SupportGitMergeTree {
		return "", nil
	}
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return "", err
	}
	mergeRef := pr.GetGitMergeRefName()

	gitRepo, err := git.OpenRepository(ctx, pr.BaseRepo)
	if err != nil {
		return "", fmt.Errorf("UpdateMergeRef: %w", err)
	}
	defer gitRepo.Close()

	headCommitID, err := gitRepo.GetRefCommitID(ctx, pr.GetGitHeadRefName())
	if err != nil {
		return "", fmt.Errorf("UpdateMergeRef: %w", err)
	}
	baseCommitID, err := gitRepo.GetRefCommitID(ctx, git.BranchPrefix+pr.BaseBranch)
	if err != nil {
		return "", fmt.Errorf("UpdateMergeRef: %w", err)
	}

	// reuse the existing merge commit so that the SHA stays stable while neither side moves
	if existing, err := gitRepo.GetCommit(ctx, mergeRef); err == nil && len(existing.Parents) == 2 &&
		existing.Parents[0].String() == baseCommitID && existing.Parents[1].String() == headCommitID {
		return existing.ID.String(), nil
	}

	mergeBase, err := git.MergeBase(ctx, pr.BaseRepo, baseCommitID, headCommitID)
	if err != nil {
		return "", fmt.Errorf("UpdateMergeRef: %w", err)
	}
	treeID, conflicted, _, err := git.MergeTree(ctx, pr.BaseRepo, baseCommitID, headCommitID, mergeBase)
	if err != nil {
		return "", fmt.Errorf("UpdateMergeRef: %w", err)
	}
	if conflicted {
		return "", git.RemoveRef(ctx, pr.BaseRepo, mergeRef)
	}

	tree, err := gitRepo.GetTree(ctx, treeID)
	if err != nil {
		return "", fmt.Errorf("UpdateMergeRef: %w", err)
	}
	sig := user_model.NewActionsUser().NewGitSig()
	commitID, err := gitRepo.CommitTree(ctx, sig, sig, tree, git.CommitTreeOpts{
		Parents:   []string{baseCommitID, headCommitID},
		Message:   fmt.Sprintf("Merge %s into %s", headCommitID, baseCommitID),
		NoGPGSign: true,
	})
	if err != nil {
		return "", fmt.Errorf("UpdateMergeRef: %w", err)
	}
	if err := git.UpdateRef(ctx, pr.BaseRepo, mergeRef, commitID.String()); err != nil {
		return "", fmt.Errorf("UpdateMergeRef: %w", err)
	}
	return commitID.String(), nil
}
