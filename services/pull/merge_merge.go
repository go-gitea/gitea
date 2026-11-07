// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"fmt"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
)

// doMergeStyleMerge merges the tracking branch into the current HEAD - which is assumed to be the staging branch (equal to the pr.BaseBranch)
func doMergeStyleMerge(ctx *mergeContext, message string) error {
	if git.DefaultFeatures().SupportGitMergeTree {
		// like "git merge", refuse to record a merge of a head the base already contains
		if isMerged, err := git.IsAncestor(ctx, ctx.tmpRepo, tmpRepoTrackingBranch, tmpRepoBaseBranch); err != nil {
			return err
		} else if isMerged {
			return fmt.Errorf("doMergeStyleMerge: the head of %v is already in its base branch", ctx.pr)
		}
		return commitMergeTree(ctx, repo_model.MergeStyleMerge, message, ctx.sig, tmpRepoBaseBranch, tmpRepoTrackingBranch)
	}
	if err := prepareTemporaryRepoForMerge(ctx); err != nil {
		return err
	}
	cmd := gitcmd.NewCommand("merge", "--no-ff", "--no-commit").AddDynamicArguments(tmpRepoTrackingBranch)
	if err := runMergeCommand(ctx, repo_model.MergeStyleMerge, cmd); err != nil {
		log.Error("%-v Unable to merge tracking into base: %v", ctx.pr, err)
		return err
	}

	if err := commitAndSignNoAuthor(ctx, message); err != nil {
		log.Error("%-v Unable to make final commit: %v", ctx.pr, err)
		return err
	}
	return nil
}
