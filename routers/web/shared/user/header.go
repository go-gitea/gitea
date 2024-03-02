// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/url"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

// prepareContextForCommonProfile store some common data into context data for user's profile related pages (including the nav menu)
// It is designed to be fast and safe to be called multiple times in one request
func prepareContextForCommonProfile(ctx *context.Context) {
	ctx.Data["IsPackageEnabled"] = setting.Packages.Enabled
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
	ctx.Data["EnableFeed"] = setting.Other.EnableFeed
	ctx.Data["FeedURL"] = ctx.ContextUser.HomeLink()
}

// PrepareContextForProfileBigAvatar set the context for big avatar view on the profile page
func PrepareContextForProfileBigAvatar(ctx *context.Context) {
	prepareContextForCommonProfile(ctx)

	ctx.Data["ShowUserEmail"] = setting.UI.ShowUserEmail && ctx.ContextUser.Email != "" && ctx.IsSigned && !ctx.ContextUser.KeepEmailPrivate
	if setting.Service.UserLocationMapURL != "" {
		ctx.Data["ContextUserLocationMapURL"] = setting.Service.UserLocationMapURL + url.QueryEscape(ctx.ContextUser.Location)
	}
	// Show OpenID URIs
	openIDs, err := user_model.GetUserOpenIDs(ctx, ctx.ContextUser.ID)
	if err != nil {
		ctx.ServerError("GetUserOpenIDs", err)
		return
	}
	ctx.Data["OpenIDs"] = openIDs

	showPrivate := ctx.IsSigned && (ctx.Doer.IsAdmin || ctx.Doer.ID == ctx.ContextUser.ID)
	orgs, err := db.Find[organization.Organization](ctx, organization.FindOrgOptions{
		UserID:         ctx.ContextUser.ID,
		IncludePrivate: showPrivate,
	})
	if err != nil {
		ctx.ServerError("FindOrgs", err)
		return
	}
	ctx.Data["Orgs"] = orgs
	ctx.Data["HasOrgsVisible"] = organization.HasOrgsVisible(ctx, orgs, ctx.Doer)

	badges, _, err := user_model.GetUserBadges(ctx, ctx.ContextUser)
	if err != nil {
		ctx.ServerError("GetUserBadges", err)
		return
	}
	ctx.Data["Badges"] = badges

	// in case the numbers are already provided by other functions, no need to query again (which is slow)
	if _, ok := ctx.Data["NumFollowers"]; !ok {
		_, ctx.Data["NumFollowers"], _ = user_model.GetUserFollowers(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{PageSize: 1, Page: 1})
	}
	if _, ok := ctx.Data["NumFollowing"]; !ok {
		_, ctx.Data["NumFollowing"], _ = user_model.GetUserFollowing(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{PageSize: 1, Page: 1})
	}
}

func FindUserProfileReadme(ctx *context.Context, doer *user_model.User) (profileDbRepo *repo_model.Repository, profileGitRepo *git.Repository, profileReadmeBlob *git.Blob, profileClose func()) {
	profileDbRepo, err := repo_model.GetRepositoryByName(ctx, ctx.ContextUser.ID, ".profile")
	if err == nil {
		perm, err := access_model.GetUserRepoPermission(ctx, profileDbRepo, doer)
		if err == nil && !profileDbRepo.IsEmpty && perm.CanRead(unit.TypeCode) {
			if profileGitRepo, err = gitrepo.OpenRepository(ctx, profileDbRepo); err != nil {
				log.Error("FindUserProfileReadme failed to OpenRepository: %v", err)
			} else {
				if commit, err := profileGitRepo.GetBranchCommit(profileDbRepo.DefaultBranch); err != nil {
					log.Error("FindUserProfileReadme failed to GetBranchCommit: %v", err)
				} else {
					profileReadmeBlob, _ = commit.GetBlobByPath("README.md")
				}
			}
		}
	} else if !repo_model.IsErrRepoNotExist(err) {
		log.Error("FindUserProfileReadme failed to GetRepositoryByName: %v", err)
	}
	return profileDbRepo, profileGitRepo, profileReadmeBlob, func() {
		if profileGitRepo != nil {
			_ = profileGitRepo.Close()
		}
	}
}

func RenderUserHeader(ctx *context.Context) {
	prepareContextForCommonProfile(ctx)

	_, _, profileReadmeBlob, profileClose := FindUserProfileReadme(ctx, ctx.Doer)
	defer profileClose()
	ctx.Data["HasProfileReadme"] = profileReadmeBlob != nil
}

func LoadHeaderCount(ctx *context.Context) error {
	prepareContextForCommonProfile(ctx)

	repoCount, err := repo_model.CountRepository(ctx, &repo_model.SearchRepoOptions{
		Actor:              ctx.Doer,
		OwnerID:            ctx.ContextUser.ID,
		Private:            ctx.IsSigned,
		Collaborate:        optional.Some(false),
		IncludeDescription: setting.UI.SearchRepoDescription,
	})
	if err != nil {
		return err
	}
	ctx.Data["RepoCount"] = repoCount

	var projectType project_model.Type
	if ctx.ContextUser.IsOrganization() {
		projectType = project_model.TypeOrganization
	} else {
		projectType = project_model.TypeIndividual
	}
	projectCount, err := db.Count[project_model.Project](ctx, project_model.SearchOptions{
		OwnerID:  ctx.ContextUser.ID,
		IsClosed: optional.Some(false),
		Type:     projectType,
	})
	if err != nil {
		return err
	}
	ctx.Data["ProjectCount"] = projectCount

	return nil
}
