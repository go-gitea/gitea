// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// doMergeStyleRebase rebaases the tracking branch on the base branch as the current HEAD with or with a merge commit to the original pr branch
func doMergeStyleRebase(ctx *mergeContext, mergeStyle repo_model.MergeStyle, message string) error {
	if err := rebaseTrackingOnToBase(ctx, mergeStyle); err != nil {
		return err
	}

	// Checkout base branch again
	if err := git.NewCommand(ctx, "checkout").AddDynamicArguments(baseBranch).
		Run(ctx.RunOpts()); err != nil {
		log.Error("git checkout base prior to merge post staging rebase %-v: %v\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
		return fmt.Errorf("git checkout base prior to merge post staging rebase  %v: %w\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
	}
	ctx.outbuf.Reset()
	ctx.errbuf.Reset()

	cmd := git.NewCommand(ctx, "merge")
	if mergeStyle == repo_model.MergeStyleRebase {
		cmd.AddArguments("--ff-only")
	} else {
		cmd.AddArguments("--no-ff", "--no-commit")
	}
	cmd.AddDynamicArguments(stagingBranch)

	// Prepare merge with commit
	if err := runMergeCommand(ctx, mergeStyle, cmd); err != nil {
		log.Error("Unable to merge staging into base: %v", err)
		return err
	}
	if mergeStyle == repo_model.MergeStyleRebaseMerge {
		if err := commitAndSignNoAuthor(ctx, message); err != nil {
			log.Error("Unable to make final commit: %v", err)
			return err
		}
	}

	return nil
}
