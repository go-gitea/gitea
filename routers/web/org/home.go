// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	html_template "html/template"
	"net/http"
	"path"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
)

const (
	tplOrgHome templates.TplName = "org/home"
)

// Home show organization home page
func Home(ctx *context.Context) {
	uname := ctx.PathParam("username")

	if strings.HasSuffix(uname, ".keys") || strings.HasSuffix(uname, ".gpg") {
		ctx.NotFound("", nil)
		return
	}

	ctx.SetPathParam("org", uname)
	context.HandleOrgAssignment(ctx)
	if ctx.Written() {
		return
	}

	home(ctx, false)
}

func Repositories(ctx *context.Context) {
	home(ctx, true)
}

func home(ctx *context.Context, viewRepositories bool) {
	org := ctx.Org.Organization

	ctx.Data["PageIsUserProfile"] = true
	ctx.Data["Title"] = org.DisplayName()

	var orderBy db.SearchOrderBy
	sortOrder := ctx.FormString("sort")
	if _, ok := repo_model.OrderByFlatMap[sortOrder]; !ok {
		sortOrder = setting.UI.ExploreDefaultSort // TODO: add new default sort order for org home?
	}
	ctx.Data["SortType"] = sortOrder
	orderBy = repo_model.OrderByFlatMap[sortOrder]

	keyword := ctx.FormTrim("q")
	ctx.Data["Keyword"] = keyword

	language := ctx.FormTrim("language")
	ctx.Data["Language"] = language

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	archived := ctx.FormOptionalBool("archived")
	ctx.Data["IsArchived"] = archived

	fork := ctx.FormOptionalBool("fork")
	ctx.Data["IsFork"] = fork

	mirror := ctx.FormOptionalBool("mirror")
	ctx.Data["IsMirror"] = mirror

	template := ctx.FormOptionalBool("template")
	ctx.Data["IsTemplate"] = template

	private := ctx.FormOptionalBool("private")
	ctx.Data["IsPrivate"] = private

	err := shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	opts := &organization.FindOrgMembersOpts{
		Doer:         ctx.Doer,
		OrgID:        org.ID,
		IsDoerMember: ctx.Org.IsMember,
		ListOptions:  db.ListOptions{Page: 1, PageSize: 25},
	}

	members, _, err := organization.FindOrgMembers(ctx, opts)
	if err != nil {
		ctx.ServerError("FindOrgMembers", err)
		return
	}
	ctx.Data["Members"] = members
	ctx.Data["Teams"] = ctx.Org.Teams
	ctx.Data["DisableNewPullMirrors"] = setting.Mirror.DisableNewPull
	ctx.Data["ShowMemberAndTeamTab"] = ctx.Org.IsMember || len(members) > 0

	currentURL := ctx.Req.URL
	queryParams := currentURL.Query()
	queryParams.Set("view_as", "member")
	ctx.Data["QueryForMember"] = html_template.URL(queryParams.Encode())
	queryParams.Set("view_as", "public")
	ctx.Data["QueryForPublic"] = html_template.URL(queryParams.Encode())

	err = shared_user.RenderOrgHeader(ctx)
	if err != nil {
		ctx.ServerError("RenderOrgHeader", err)
		return
	}
	isBothProfilesExist := ctx.Data["HasPublicProfileReadme"] == true && ctx.Data["HasPrivateProfileReadme"] == true

	isViewerMember := ctx.FormString("view_as")
	ctx.Data["IsViewerMember"] = isViewerMember == "member"

	profileType := "Public"
	if isViewerMember == "member" {
		profileType = "Private"
	}

	if !isBothProfilesExist {
		if !prepareOrgProfileReadme(ctx, viewRepositories, "Public") {
			if !prepareOrgProfileReadme(ctx, viewRepositories, "Private") {
				ctx.Data["PageIsViewRepositories"] = true
			}
		}
	} else {
		if !prepareOrgProfileReadme(ctx, viewRepositories, profileType) {
			ctx.Data["PageIsViewRepositories"] = true
		}
	}

	var (
		repos []*repo_model.Repository
		count int64
	)
	repos, count, err = repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			PageSize: setting.UI.User.RepoPagingNum,
			Page:     page,
		},
		Keyword:            keyword,
		OwnerID:            org.ID,
		OrderBy:            orderBy,
		Private:            ctx.IsSigned,
		Actor:              ctx.Doer,
		Language:           language,
		IncludeDescription: setting.UI.SearchRepoDescription,
		Archived:           archived,
		Fork:               fork,
		Mirror:             mirror,
		Template:           template,
		IsPrivate:          private,
	})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}

	ctx.Data["Repos"] = repos
	ctx.Data["Total"] = count

	pager := context.NewPagination(int(count), setting.UI.User.RepoPagingNum, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplOrgHome)
}

func prepareOrgProfileReadme(ctx *context.Context, viewRepositories bool, profileType string) bool {
	profileDbRepo, profileGitRepo, profileReadme, profileClose := shared_user.FindUserProfileReadme(ctx, ctx.Doer, profileType)
	defer profileClose()
	ctx.Data[fmt.Sprintf("Has%sProfileReadme", profileType)] = profileReadme != nil

	if profileGitRepo == nil || profileReadme == nil || viewRepositories {
		return false
	}

	if bytes, err := profileReadme.GetBlobContent(setting.UI.MaxDisplayFileSize); err != nil {
		log.Error("failed to GetBlobContent for %s profile readme: %v", profileType, err)
	} else {
		rctx := renderhelper.NewRenderContextRepoFile(ctx, profileDbRepo, renderhelper.RepoFileOptions{
			CurrentRefPath: path.Join("branch", util.PathEscapeSegments(profileDbRepo.DefaultBranch)),
		})
		if profileContent, err := markdown.RenderString(rctx, bytes); err != nil {
			log.Error("failed to RenderString for %s profile readme: %v", profileType, err)
		} else {
			ctx.Data[fmt.Sprintf("%sProfileReadme", profileType)] = profileContent
		}
	}
	return true
}
