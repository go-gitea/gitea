// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"
	"html/template"
	"net/url"
	"path"
	"slices"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	project_model "gitea.dev/models/project"
	"gitea.dev/models/renderhelper"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	"gitea.dev/services/ownerrepo"
)

// prepareContextForProfileBigAvatar set the context for big avatar view on the profile page
func prepareContextForProfileBigAvatar(ctx *context.Context) {
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

	orgs, err := db.Find[organization.Organization](ctx, organization.FindOrgOptions{
		UserID:            ctx.ContextUser.ID,
		IncludeVisibility: organization.DoerViewOtherVisibility(ctx.Doer, ctx.ContextUser),
		ListOptions: db.ListOptions{
			Page: 1,
			// query one more result (without a separate counting) to see whether we need to add the "show more orgs" link
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
		ctx.Data["UserBlocking"], err = user_model.GetBlocking(ctx, ctx.Doer.ID, ctx.ContextUser.ID)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			ctx.ServerError("GetBlocking", err)
		}
	}
}

type ProfileReadme struct {
	Repo    *repo_model.Repository
	Blob    *git.Blob
	TreeDir string
}

func (readme *ProfileReadme) Render(ctx *context.Context) (template.HTML, error) {
	content, err := readme.Blob.GetBlobContent(ctx, setting.UI.MaxDisplayFileSize)
	if err != nil {
		return "", err
	}
	rctx := renderhelper.NewRenderContextRepoFile(ctx, readme.Repo, renderhelper.RepoFileOptions{
		CurrentRefSubURL: git.RefNameFromBranch(readme.Repo.DefaultBranch).RefWebLinkPath(),
		CurrentTreePath:  readme.TreeDir,
	})
	return markdown.RenderString(rctx, content)
}

func FindOwnerProfileReadme(ctx *context.Context, doer *user_model.User) *ProfileReadme {
	return findProfileReadme(ctx, doer, slices.Concat([]string{RepoNameProfile}, ownerrepo.DefaultsRepoNames))
}

func findProfileReadme(ctx *context.Context, doer *user_model.User, repoNames []string) *ProfileReadme {
	var readme *ProfileReadme
	ownerrepo.Find(ctx, doer, ctx.ContextUser, repoNames, func(found *ownerrepo.Repo) bool {
		treeDir := util.Iif(slices.Contains(ownerrepo.DefaultsRepoNames, found.Repo.LowerName), "profile", "")
		if blob, err := found.Commit.GetBlobByPath(ctx, found.GitRepo, path.Join(treeDir, "README.md")); err == nil {
			readme = &ProfileReadme{Repo: found.Repo, Blob: blob, TreeDir: treeDir}
		}
		return readme != nil
	})
	return readme
}

type PrepareOwnerHeaderResult struct {
	ProfilePublicReadme  *ProfileReadme
	ProfilePrivateReadme *ProfileReadme
	HasOrgProfileReadme  bool
}

const (
	RepoNameProfilePrivate = ".profile-private"
	RepoNameProfile        = ".profile"
)

func RenderUserOrgHeader(ctx *context.Context) (result *PrepareOwnerHeaderResult, err error) {
	ctx.Data["IsPackageEnabled"] = setting.Packages.Enabled
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
	ctx.Data["EnableFeed"] = setting.Other.EnableFeed
	ctx.Data["FeedURL"] = ctx.ContextUser.HomeLink()

	if err := loadHeaderCount(ctx); err != nil {
		return nil, err
	}

	result = &PrepareOwnerHeaderResult{}
	if ctx.ContextUser.IsOrganization() {
		result.ProfilePublicReadme = FindOwnerProfileReadme(ctx, ctx.Doer)
		result.ProfilePrivateReadme = findProfileReadme(ctx, ctx.Doer, []string{RepoNameProfilePrivate})
		result.HasOrgProfileReadme = result.ProfilePublicReadme != nil || result.ProfilePrivateReadme != nil
		ctx.Data["HasOrgProfileReadme"] = result.HasOrgProfileReadme // many pages need it to show the "overview" tab
	} else {
		ctx.Data["HasUserProfileReadme"] = FindOwnerProfileReadme(ctx, ctx.Doer) != nil
		prepareContextForProfileBigAvatar(ctx)
	}
	return result, nil
}

func loadHeaderCount(ctx *context.Context) error {
	repoCount, err := repo_model.CountRepository(ctx, repo_model.SearchRepoOptions{
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
