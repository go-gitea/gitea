// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"strings"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/log"
)

// doMergeStyleMerge merges the tracking branch into the current HEAD - which is assumed to be the staging branch (equal to the pr.BaseBranch)
func doMergeStyleMerge(ctx *mergeContext, message string) error {
	cmd := gitcmd.NewCommand("merge", "--no-ff", "--no-commit").AddDynamicArguments(trackingBranch)
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

// CalcMergeBase calculates the merge base for a pull request.
func CalcMergeBase(ctx context.Context, pr *issues_model.PullRequest) (string, error) {
	repoPath := pr.BaseRepo.RepoPath()
	if pr.HasMerged {
		mergeBase, _, err := gitcmd.NewCommand("merge-base").AddDashesAndList(pr.MergedCommitID+"^", pr.GetGitHeadRefName()).
			RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath})
		return strings.TrimSpace(mergeBase), err
	}

	mergeBase, _, err := gitcmd.NewCommand("merge-base").AddDashesAndList(pr.BaseBranch, pr.GetGitHeadRefName()).
		RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath})
	if err != nil {
		var err2 error
		mergeBase, _, err2 = gitcmd.NewCommand("rev-parse").AddDynamicArguments(git.BranchPrefix+pr.BaseBranch).
			RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath})
		if err2 != nil {
			log.Error("Unable to get merge base for PR ID %d, Index %d in %s/%s. Error: %v & %v", pr.ID, pr.Index, pr.BaseRepo.OwnerName, pr.BaseRepo.Name, err, err2)
			return "", err2
		}
	}
	return strings.TrimSpace(mergeBase), nil
}
