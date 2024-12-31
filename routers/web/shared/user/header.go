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
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
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

	ctx.Data["IsFollowing"] = ctx.Doer != nil && user_model.IsFollowing(ctx, ctx.Doer.ID, ctx.ContextUser.ID)
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
	if len(ctx.ContextUser.Description) != 0 {
		content, err := markdown.RenderString(markup.NewRenderContext(ctx).WithMetas(markup.ComposeSimpleDocumentMetas()), ctx.ContextUser.Description)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
		ctx.Data["RenderedDescription"] = content
	}

	showPrivate := ctx.IsSigned && (ctx.Doer.IsAdmin || ctx.Doer.ID == ctx.ContextUser.ID)
	orgs, err := db.Find[organization.Organization](ctx, organization.FindOrgOptions{
		UserID:         ctx.ContextUser.ID,
		IncludePrivate: showPrivate,
		ListOptions: db.ListOptions{
			Page: 1,
			// query one more results (without a separate counting) to see whether we need to add the "show more orgs" link
			PageSize: setting.UI.User.OrgPagingNum + 1,
		},
	})
	if err != nil {
		ctx.ServerError("FindOrgs", err)
		return
	}
	if len(orgs) > setting.UI.User.OrgPagingNum {
		orgs = orgs[:setting.UI.User.OrgPagingNum]
		ctx.Data["ShowMoreOrgs"] = true
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

	if ctx.Doer != nil {
		if block, err := user_model.GetBlocking(ctx, ctx.Doer.ID, ctx.ContextUser.ID); err != nil {
			ctx.ServerError("GetBlocking", err)
		} else {
			ctx.Data["UserBlocking"] = block
		}
	}
}

func FindOwnerProfileReadme(ctx *context.Context, doer *user_model.User, optProfileRepoName ...string) (profileDbRepo *repo_model.Repository, profileReadmeBlob *git.Blob) {
	profileRepoName := util.OptionalArg(optProfileRepoName, RepoNameProfile)
	profileDbRepo, err := repo_model.GetRepositoryByName(ctx, ctx.ContextUser.ID, profileRepoName)
	if err != nil {
		if !repo_model.IsErrRepoNotExist(err) {
			log.Error("FindOwnerProfileReadme failed to GetRepositoryByName: %v", err)
		}
		return nil, nil
	}

	perm, err := access_model.GetUserRepoPermission(ctx, profileDbRepo, doer)
	if err != nil {
		log.Error("FindOwnerProfileReadme failed to GetRepositoryByName: %v", err)
		return nil, nil
	}
	if profileDbRepo.IsEmpty || !perm.CanRead(unit.TypeCode) {
		return nil, nil
	}

	profileGitRepo, err := gitrepo.RepositoryFromRequestContextOrOpen(ctx, profileDbRepo)
	if err != nil {
		log.Error("FindOwnerProfileReadme failed to OpenRepository: %v", err)
		return nil, nil
	}

	commit, err := profileGitRepo.GetBranchCommit(profileDbRepo.DefaultBranch)
	if err != nil {
		log.Error("FindOwnerProfileReadme failed to GetBranchCommit: %v", err)
		return nil, nil
	}

	profileReadmeBlob, _ = commit.GetBlobByPath("README.md") // no need to handle this error
	return profileDbRepo, profileReadmeBlob
}

func RenderUserHeader(ctx *context.Context) {
	prepareContextForCommonProfile(ctx)

	_, profileReadmeBlob := FindOwnerProfileReadme(ctx, ctx.Doer)
	ctx.Data["HasUserProfileReadme"] = profileReadmeBlob != nil
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

const (
	RepoNameProfilePrivate = ".profile-private"
	RepoNameProfile        = ".profile"
)

type PrepareOrgHeaderResult struct {
	ProfilePublicRepo        *repo_model.Repository
	ProfilePublicReadmeBlob  *git.Blob
	ProfilePrivateRepo       *repo_model.Repository
	ProfilePrivateReadmeBlob *git.Blob
	HasOrgProfileReadme      bool
}

func PrepareOrgHeader(ctx *context.Context) (result *PrepareOrgHeaderResult, err error) {
	if err = LoadHeaderCount(ctx); err != nil {
		return nil, err
	}

	result = &PrepareOrgHeaderResult{}
	result.ProfilePublicRepo, result.ProfilePublicReadmeBlob = FindOwnerProfileReadme(ctx, ctx.Doer)
	result.ProfilePrivateRepo, result.ProfilePrivateReadmeBlob = FindOwnerProfileReadme(ctx, ctx.Doer, RepoNameProfilePrivate)
	result.HasOrgProfileReadme = result.ProfilePublicReadmeBlob != nil || result.ProfilePrivateReadmeBlob != nil
	ctx.Data["HasOrgProfileReadme"] = result.HasOrgProfileReadme // many pages need it to show the "overview" tab
	return result, nil
}
