// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	git_model "code.gitea.io/gitea/models/git"
	access_model "code.gitea.io/gitea/models/perm/access"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/context"
	repo_service "code.gitea.io/gitea/services/repository"
)

type RecentBranchesPromptDataStruct struct {
	RecentlyPushedNewBranches []*git_model.RecentlyPushedNewBranch
}

func prepareRecentlyPushedNewBranches(ctx *context.Context) {
	if ctx.Doer == nil {
		return
	}
	baseRepo, err := ctx.Repo.Repository.GetPullRequestDefaultBaseRepo(ctx)
	if err != nil {
		log.Error("GetPullRequestDefaultBaseRepo: %v", err)
		return
	}
	if baseRepo == nil {
		return
	}

	opts := git_model.FindRecentlyPushedNewBranchesOptions{
		Repo:     ctx.Repo.Repository,
		BaseRepo: baseRepo,
	}

	baseRepoPerm, err := access_model.GetDoerRepoPermission(ctx, opts.BaseRepo, ctx.Doer)
	if err != nil {
		log.Error("GetDoerRepoPermission: %v", err)
		return
	}
	if !opts.Repo.CanContentChange() || !opts.BaseRepo.CanContentChange() {
		return
	}
	if !opts.BaseRepo.UnitEnabled(ctx, unit_model.TypePullRequests) || !baseRepoPerm.CanRead(unit_model.TypePullRequests) {
		return
	}

	var finalBranches []*git_model.RecentlyPushedNewBranch
	branches, err := git_model.FindRecentlyPushedNewBranches(ctx, ctx.Doer, opts)
	if err != nil {
		log.Error("FindRecentlyPushedNewBranches failed: %v", err)
		return
	}

	for _, branch := range branches {
		divergingInfo, err := repo_service.GetBranchDivergingInfo(ctx,
			branch.BranchRepo, branch.BranchName, // "base" repo for diverging info
			opts.BaseRepo, opts.BaseRepo.DefaultBranch, // "head" repo for diverging info
		)
		if err != nil {
			log.Error("GetBranchDivergingInfo failed: %v", err)
			continue
		}
		branchRepoHasNewCommits := divergingInfo.BaseHasNewCommits
		baseRepoCommitsBehind := divergingInfo.HeadCommitsBehind
		if branchRepoHasNewCommits || baseRepoCommitsBehind > 0 {
			finalBranches = append(finalBranches, branch)
		}
	}
	if len(finalBranches) > 0 {
		ctx.Data["RecentBranchesPromptData"] = RecentBranchesPromptDataStruct{finalBranches}
	}
}
