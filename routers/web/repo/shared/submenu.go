// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/optional"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/services/context"
	repo_service "code.gitea.io/gitea/services/repository"
)

// PrepareRepoSubMenu prepares data for repository template repo/sub_menu.tmpl
func PrepareRepoSubMenu(ctx *context.Context) bool {
	if !prepareSubmenuBranch(ctx) {
		return false
	}

	if !PrepareSubmenuTag(ctx) {
		return false
	}

	if !prepareSubmenuCommit(ctx) {
		return false
	}

	// only show license on repository's home page
	if ctx.Data["PageIsRepoHome"] == true {
		if !prepareSubmenuLicense(ctx) {
			return false
		}
	}

	return true
}

func prepareSubmenuLicense(ctx *context.Context) bool {
	repoLicenses, err := repo_model.GetRepoLicenses(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("GetRepoLicenses", err)
		return false
	}
	ctx.Data["DetectedRepoLicenses"] = repoLicenses.StringList()
	ctx.Data["LicenseFileName"] = repo_service.LicenseFileName
	return true
}

func prepareSubmenuBranch(ctx *context.Context) bool {
	branchOpts := git_model.FindBranchOptions{
		RepoID:          ctx.Repo.Repository.ID,
		IsDeletedBranch: optional.Some(false),
		ListOptions:     db.ListOptionsAll,
	}
	branchesTotal, err := db.Count[git_model.Branch](ctx, branchOpts)
	if err != nil {
		ctx.ServerError("CountBranches", err)
		return false
	}

	// non-empty repo should have at least 1 branch, so this repository's branches haven't been synced yet
	if branchesTotal == 0 { // fallback to do a sync immediately
		branchesTotal, err = repo_module.SyncRepoBranches(ctx, ctx.Repo.Repository.ID, 0)
		if err != nil {
			ctx.ServerError("SyncRepoBranches", err)
			return false
		}
	}

	ctx.Data["BranchesCount"] = branchesTotal
	return true
}

func PrepareSubmenuTag(ctx *context.Context) bool {
	var err error
	ctx.Data["NumTags"], err = db.Count[repo_model.Release](ctx, repo_model.FindReleasesOptions{
		IncludeDrafts: true,
		IncludeTags:   true,
		HasSha1:       optional.Some(true), // only draft releases which are created with existing tags
		RepoID:        ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.ServerError("GetReleaseCountByRepoID", err)
		return false
	}
	return true
}

func prepareSubmenuCommit(ctx *context.Context) bool {
	var err error
	ctx.Repo.CommitsCount, err = ctx.Repo.GetCommitsCount()
	if err != nil {
		ctx.ServerError("GetCommitsCount", err)
		return false
	}
	ctx.Data["CommitsCount"] = ctx.Repo.CommitsCount
	ctx.Repo.GitRepo.LastCommitCache = git.NewLastCommitCache(ctx.Repo.CommitsCount, ctx.Repo.Repository.FullName(), ctx.Repo.GitRepo, cache.GetCache())
	return true
}
