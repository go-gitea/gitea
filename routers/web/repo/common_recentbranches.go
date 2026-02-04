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
	if err := ctx.Repo.Repository.GetBaseRepo(ctx); err != nil {
		log.Error("GetBaseRepo: %v", err)
		return
	}

	opts := git_model.FindRecentlyPushedNewBranchesOptions{
		Repo:     ctx.Repo.Repository,
		BaseRepo: ctx.Repo.Repository,
	}
	if ctx.Repo.Repository.IsFork {
		opts.BaseRepo = ctx.Repo.Repository.BaseRepo
	}

	baseRepoPerm, err := access_model.GetUserRepoPermission(ctx, opts.BaseRepo, ctx.Doer)
	if err != nil {
		log.Error("GetUserRepoPermission: %v", err)
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
