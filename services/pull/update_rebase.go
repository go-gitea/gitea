// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"
	"strings"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
)

// updateHeadByRebaseOnToBase handles updating a PR's head branch by rebasing it on the PR current base branch
func updateHeadByRebaseOnToBase(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User) error {
	// "Clone" base repo and add the cache headers for the head repo and branch
	mergeCtx, cancel, err := createTemporaryRepoForMerge(ctx, pr, doer, "")
	if err != nil {
		return err
	}
	defer cancel()

	// Determine the old merge-base before the rebase - we use this for LFS push later on
	oldMergeBase, _, _ := git.NewCommand(ctx, "merge-base").AddDashesAndList(baseBranch, trackingBranch).RunStdString(&git.RunOpts{Dir: mergeCtx.tmpBasePath})
	oldMergeBase = strings.TrimSpace(oldMergeBase)

	// Rebase the tracking branch on to the base as the staging branch
	if err := rebaseTrackingOnToBase(mergeCtx, repo_model.MergeStyleRebaseUpdate); err != nil {
		return err
	}

	if setting.LFS.StartServer {
		// Now we need to ensure that the head repository contains any LFS objects between the new base and the old mergebase
		// It's questionable about where this should go - either after or before the push
		// I think in the interests of data safety - failures to push to the lfs should prevent
		// the push as you can always re-rebase.
		if err := LFSPush(ctx, mergeCtx.tmpBasePath, baseBranch, oldMergeBase, &issues_model.PullRequest{
			HeadRepoID: pr.BaseRepoID,
			BaseRepoID: pr.HeadRepoID,
		}); err != nil {
			log.Error("Unable to push lfs objects between %s and %s up to head branch in %-v: %v", baseBranch, oldMergeBase, pr, err)
			return err
		}
	}

	// Now determine who the pushing author should be
	var headUser *user_model.User
	if err := pr.HeadRepo.LoadOwner(ctx); err != nil {
		if !user_model.IsErrUserNotExist(err) {
			log.Error("Can't find user: %d for head repository in %-v - %v", pr.HeadRepo.OwnerID, pr, err)
			return err
		}
		log.Error("Can't find user: %d for head repository in %-v - defaulting to doer: %-v - %v", pr.HeadRepo.OwnerID, pr, doer, err)
		headUser = doer
	} else {
		headUser = pr.HeadRepo.Owner
	}

	pushCmd := git.NewCommand(ctx, "push", "-f", "head_repo").
		AddDynamicArguments(stagingBranch + ":" + git.BranchPrefix + pr.HeadBranch)

	// Push back to the head repository.
	// TODO: this cause an api call to "/api/internal/hook/post-receive/...",
	//       that prevents us from doint the whole merge in one db transaction
	mergeCtx.outbuf.Reset()
	mergeCtx.errbuf.Reset()

	if err := pushCmd.Run(&git.RunOpts{
		Env: repo_module.FullPushingEnvironment(
			headUser,
			doer,
			pr.HeadRepo,
			pr.HeadRepo.Name,
			pr.ID,
		),
		Dir:    mergeCtx.tmpBasePath,
		Stdout: mergeCtx.outbuf,
		Stderr: mergeCtx.errbuf,
	}); err != nil {
		if strings.Contains(mergeCtx.errbuf.String(), "non-fast-forward") {
			return &git.ErrPushOutOfDate{
				StdOut: mergeCtx.outbuf.String(),
				StdErr: mergeCtx.errbuf.String(),
				Err:    err,
			}
		} else if strings.Contains(mergeCtx.errbuf.String(), "! [remote rejected]") {
			err := &git.ErrPushRejected{
				StdOut: mergeCtx.outbuf.String(),
				StdErr: mergeCtx.errbuf.String(),
				Err:    err,
			}
			err.GenerateMessage()
			return err
		}
		return fmt.Errorf("git push: %s", mergeCtx.errbuf.String())
	}
	mergeCtx.outbuf.Reset()
	mergeCtx.errbuf.Reset()

	return nil
}
