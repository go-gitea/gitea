// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"fmt"
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/forms"
)

const (
	tplStarListRepos base.TplName = "user/starlist/repos"
)

func ShowStarList(ctx *context.Context) {
	if setting.Repository.DisableStarLists {
		ctx.NotFound("", fmt.Errorf(""))
		return
	}

	shared_user.PrepareContextForProfileBigAvatar(ctx)

	err := shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	name := ctx.Params("name")

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	keyword := ctx.FormTrim("q")
	ctx.Data["Keyword"] = keyword

	language := ctx.FormTrim("language")
	ctx.Data["Language"] = language

	pagingNum := setting.UI.User.RepoPagingNum

	starList, err := repo_model.GetStarListByName(ctx, ctx.ContextUser.ID, name)
	if err != nil {
		if repo_model.IsErrStarListNotFound(err) {
			ctx.NotFound("", fmt.Errorf(""))
		} else {
			ctx.ServerError("GetStarListByName", err)
		}
		return
	}

	if !starList.HasAccess(ctx.Doer) {
		ctx.NotFound("", fmt.Errorf(""))
		return
	}

	err = starList.LoadUser(ctx)
	if err != nil {
		ctx.ServerError("LoadUser", err)
		return
	}

	repos, count, err := repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{Actor: ctx.Doer, StarListID: starList.ID, Keyword: keyword, Language: language})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}

	ctx.Data["Repos"] = repos
	ctx.Data["Total"] = count

	pager := context.NewPagination(int(count), pagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.Data["TabName"] = "stars"
	ctx.Data["Title"] = starList.Name
	ctx.Data["CurrentStarList"] = starList
	ctx.Data["PageIsProfileStarList"] = true
	ctx.Data["StarListEditRedirect"] = starList.Link()
	ctx.Data["ShowStarListEditButtons"] = ctx.ContextUser.IsSameUser(ctx.Doer)
	ctx.Data["EditStarListURL"] = fmt.Sprintf("%s/-/starlist_edit", ctx.ContextUser.HomeLink())

	ctx.HTML(http.StatusOK, tplStarListRepos)
}

func editStarList(ctx *context.Context, form forms.EditStarListForm) {
	starList, err := repo_model.GetStarListByID(ctx, form.ID)
	if err != nil {
		ctx.ServerError("GetStarListByID", err)
		return
	}

	err = starList.LoadUser(ctx)
	if err != nil {
		ctx.ServerError("LoadUser", err)
		return
	}

	// Check if the doer is the owner of the list
	if ctx.Doer.ID != starList.UserID {
		ctx.Flash.Error("Not the same user")
		ctx.Redirect(starList.Link())
		return
	}

	err = starList.EditData(ctx, form.Name, form.Description, form.Private)
	if err != nil {
		if repo_model.IsErrStarListExists(err) {
			ctx.Flash.Error(ctx.Tr("starlist.name_exists_error", form.Name))
			ctx.Redirect(starList.Link())
		} else {
			ctx.ServerError("EditData", err)
		}
		return
	}

	ctx.Redirect(starList.Link())
}

func addStarList(ctx *context.Context, form forms.EditStarListForm) {
	starList, err := repo_model.CreateStarList(ctx, ctx.Doer.ID, form.Name, form.Description, form.Private)
	if err != nil {
		if repo_model.IsErrStarListExists(err) {
			ctx.Flash.Error(ctx.Tr("starlist.name_exists_error", form.Name))
			ctx.Redirect(form.CurrentURL)
		} else {
			ctx.ServerError("CreateStarList", err)
		}
		return
	}

	err = starList.LoadUser(ctx)
	if err != nil {
		ctx.ServerError("LoadUser", err)
		return
	}

	ctx.Redirect(starList.Link())
}

func deleteStarList(ctx *context.Context, form forms.EditStarListForm) {
	starList, err := repo_model.GetStarListByID(ctx, form.ID)
	if err != nil {
		ctx.ServerError("GetStarListByID", err)
		return
	}

	// Check if the doer is the owner of the list
	if ctx.Doer.ID != starList.UserID {
		ctx.Flash.Error("Not the same user")
		ctx.Redirect(form.CurrentURL)
		return
	}

	err = repo_model.DeleteStarListByID(ctx, starList.ID)
	if err != nil {
		ctx.ServerError("GetStarListByID", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("starlist.delete_success_message", starList.Name))

	ctx.Redirect(fmt.Sprintf("%s?tab=stars", ctx.ContextUser.HomeLink()))
}

func EditStarListPost(ctx *context.Context) {
	form := *web.GetForm(ctx).(*forms.EditStarListForm)

	switch form.Action {
	case "edit":
		editStarList(ctx, form)
	case "add":
		addStarList(ctx, form)
	case "delete":
		deleteStarList(ctx, form)
	default:
		ctx.Flash.Error(fmt.Sprintf("Unknown action %s", form.Action))
		ctx.Redirect(form.CurrentURL)
	}
}
