// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
)

const (
	tplOrgHome base.TplName = "org/home"
)

// Home show organization home page
func Home(ctx *context.Context) {
	uname := ctx.PathParam(":username")

	if strings.HasSuffix(uname, ".keys") || strings.HasSuffix(uname, ".gpg") {
		ctx.NotFound("", nil)
		return
	}

	ctx.SetPathParam(":org", uname)
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
		OrgID:       org.ID,
		PublicOnly:  ctx.Org.PublicMemberOnly,
		ListOptions: db.ListOptions{Page: 1, PageSize: 25},
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

	if !prepareOrgProfileReadme(ctx, viewRepositories) {
		ctx.Data["PageIsViewRepositories"] = true
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
	pager.SetDefaultParams(ctx)
	pager.AddParamString("language", language)
	if archived.Has() {
		pager.AddParamString("archived", fmt.Sprint(archived.Value()))
	}
	if fork.Has() {
		pager.AddParamString("fork", fmt.Sprint(fork.Value()))
	}
	if mirror.Has() {
		pager.AddParamString("mirror", fmt.Sprint(mirror.Value()))
	}
	if template.Has() {
		pager.AddParamString("template", fmt.Sprint(template.Value()))
	}
	if private.Has() {
		pager.AddParamString("private", fmt.Sprint(private.Value()))
	}
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplOrgHome)
}

func prepareOrgProfileReadme(ctx *context.Context, viewRepositories bool) bool {
	profileDbRepo, profileGitRepo, profileReadme, profileClose := shared_user.FindUserProfileReadme(ctx, ctx.Doer)
	defer profileClose()
	ctx.Data["HasProfileReadme"] = profileReadme != nil

	if profileGitRepo == nil || profileReadme == nil || viewRepositories {
		return false
	}

	if bytes, err := profileReadme.GetBlobContent(setting.UI.MaxDisplayFileSize); err != nil {
		log.Error("failed to GetBlobContent: %v", err)
	} else {
		if profileContent, err := markdown.RenderString(&markup.RenderContext{
			Ctx:     ctx,
			GitRepo: profileGitRepo,
			Links: markup.Links{
				// Pass repo link to markdown render for the full link of media elements.
				// The profile of default branch would be shown.
				Base:       profileDbRepo.Link(),
				BranchPath: path.Join("branch", util.PathEscapeSegments(profileDbRepo.DefaultBranch)),
			},
			Metas: map[string]string{"mode": "document"},
		}, bytes); err != nil {
			log.Error("failed to RenderString: %v", err)
		} else {
			ctx.Data["ProfileReadme"] = profileContent
		}
	}

	ctx.Data["PageIsViewOverview"] = true
	return true
}
