// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
)

// Tags render the page to protect tags
func Tags(ctx *context.Context) {
	if setTagsContext(ctx) != nil {
		return
	}

	ctx.HTML(http.StatusOK, tplTags)
}

// TagPost handles creation of a protect tag
func TagPost(ctx *context.Context) {
	if setTagsContext(ctx) != nil {
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplTags)
		return
	}

	repo := ctx.Repo.Repository
	form := web.GetForm(ctx).(*forms.ProtectTagForm)

	pt := &models.ProtectedTag{
		RepoID:      repo.ID,
		NamePattern: form.NamePattern,
	}

	if strings.TrimSpace(form.WhitelistUsers) != "" {
		pt.WhitelistUserIDs, _ = base.StringsToInt64s(strings.Split(form.WhitelistUsers, ","))
	}
	if strings.TrimSpace(form.WhitelistTeams) != "" {
		pt.WhitelistTeamIDs, _ = base.StringsToInt64s(strings.Split(form.WhitelistTeams, ","))
	}

	if err := models.InsertProtectedTag(pt); err != nil {
		ctx.ServerError("InsertProtectedTag", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
	ctx.Redirect(setting.AppSubURL + ctx.Req.URL.Path)
}

// TagDelete handles deletion of a protected tag
func TagDelete(ctx *context.Context) {
	pt, err := selectProtectedTagByContext(ctx)
	if err != nil {
		ctx.NotFound("", nil)
		return
	}

	if err := models.DeleteProtectedTag(pt); err != nil {
		ctx.ServerError("DeleteProtectedTag", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
	ctx.Redirect(ctx.Repo.Repository.Link() + "/settings/tags")
}

func setTagsContext(ctx *context.Context) error {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsTags"] = true

	protectedTags, err := ctx.Repo.Repository.GetProtectedTags()
	if err != nil {
		ctx.ServerError("GetProtectedTags", err)
		return err
	}
	ctx.Data["ProtectedTags"] = protectedTags

	users, err := ctx.Repo.Repository.GetReaders()
	if err != nil {
		ctx.ServerError("Repo.Repository.GetReaders", err)
		return err
	}
	ctx.Data["Users"] = users

	if ctx.Repo.Owner.IsOrganization() {
		teams, err := ctx.Repo.Owner.TeamsWithAccessToRepo(ctx.Repo.Repository.ID, models.AccessModeRead)
		if err != nil {
			ctx.ServerError("Repo.Owner.TeamsWithAccessToRepo", err)
			return err
		}
		ctx.Data["Teams"] = teams
	}

	return nil
}

func selectProtectedTagByContext(ctx *context.Context) (*models.ProtectedTag, error) {
	pts, err := ctx.Repo.Repository.GetProtectedTags()
	if err != nil {
		return nil, err
	}

	id, _ := strconv.ParseInt(ctx.Query("id"), 10, 64)

	for _, pt := range pts {
		if pt.ID == id {
			return pt, nil
		}
	}

	return nil, fmt.Errorf("ProtectedTag[%v] not associated to repository %v", id, ctx.Repo.Repository)
}
