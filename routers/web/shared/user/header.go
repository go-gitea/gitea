// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
)

// RenderProfileBigAvatar set the context for big avatar view on repo
func RenderProfileBigAvatar(ctx *context.Context) {
	// check view permissions
	if !user_model.IsUserVisibleToViewer(ctx, ctx.ContextUser, ctx.Doer) {
		ctx.NotFound("user", fmt.Errorf(ctx.ContextUser.Name))
		return
	}

	// advertise feed via meta tag
	ctx.Data["FeedURL"] = ctx.ContextUser.HomeLink()

	// Show OpenID URIs
	openIDs, err := user_model.GetUserOpenIDs(ctx.ContextUser.ID)
	if err != nil {
		ctx.ServerError("GetUserOpenIDs", err)
		return
	}

	var isFollowing bool
	if ctx.Doer != nil {
		isFollowing = user_model.IsFollowing(ctx.Doer.ID, ctx.ContextUser.ID)
	}

	ctx.Data["ContextUser"] = ctx.ContextUser
	ctx.Data["OpenIDs"] = openIDs
	ctx.Data["IsFollowing"] = isFollowing

	if len(ctx.ContextUser.Description) != 0 {
		content, err := markdown.RenderString(&markup.RenderContext{
			URLPrefix: ctx.Repo.RepoLink,
			Metas:     map[string]string{"mode": "document"},
			GitRepo:   ctx.Repo.GitRepo,
			Ctx:       ctx,
		}, ctx.ContextUser.Description)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
		ctx.Data["RenderedDescription"] = content
	}
	showPrivate := ctx.IsSigned && (ctx.Doer.IsAdmin || ctx.Doer.ID == ctx.ContextUser.ID)
	orgs, err := organization.FindOrgs(organization.FindOrgOptions{
		UserID:         ctx.ContextUser.ID,
		IncludePrivate: showPrivate,
	})
	if err != nil {
		ctx.ServerError("FindOrgs", err)
		return
	}
	ctx.Data["Orgs"] = orgs
	ctx.Data["HasOrgsVisible"] = organization.HasOrgsVisible(orgs, ctx.Doer)

	badges, _, err := user_model.GetUserBadges(ctx, ctx.ContextUser)
	if err != nil {
		ctx.ServerError("GetUserBadges", err)
		return
	}
	ctx.Data["Badges"] = badges

	pagingNum := setting.UI.User.RepoPagingNum
	page := ctx.FormInt("page")
	_, numFollowers, err := user_model.GetUserFollowers(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{
		PageSize: pagingNum,
		Page:     page,
	})
	if err != nil {
		ctx.ServerError("GetUserFollowers", err)
		return
	}
	ctx.Data["NumFollowers"] = numFollowers
	_, numFollowing, err := user_model.GetUserFollowing(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{
		PageSize: pagingNum,
		Page:     page,
	})
	if err != nil {
		ctx.ServerError("GetUserFollowing", err)
		return
	}
	ctx.Data["NumFollowing"] = numFollowing
	ctx.Data["ShowUserEmail"] = setting.UI.ShowUserEmail && ctx.ContextUser.Email != "" && ctx.IsSigned && !ctx.ContextUser.KeepEmailPrivate
	ctx.Data["EnableFeed"] = setting.Other.EnableFeed
}

func RenderUserHeader(ctx *context.Context) {
	ctx.Data["IsProjectEnabled"] = true
	ctx.Data["IsPackageEnabled"] = setting.Packages.Enabled
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
	ctx.Data["ContextUser"] = ctx.ContextUser
	tab := ctx.FormString("tab")
	ctx.Data["TabName"] = tab
	repo, err := repo_model.GetRepositoryByName(ctx.ContextUser.ID, ".profile")
	if err == nil && !repo.IsEmpty {
		gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return
		}
		defer gitRepo.Close()
		commit, err := gitRepo.GetBranchCommit(repo.DefaultBranch)
		if err != nil {
			ctx.ServerError("GetBranchCommit", err)
			return
		}
		blob, err := commit.GetBlobByPath("README.md")
		if err == nil && blob != nil {
			ctx.Data["ProfileReadme"] = true
		}
	}
}
