// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"net/http"
	"strings"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
)

const (
	tplTags base.TplName = "repo/settings/tags"
)

// Tags render the page to protect tags
func ProtectedTags(ctx *context.Context) {
	if setTagsContext(ctx) != nil {
		return
	}

	ctx.HTML(http.StatusOK, tplTags)
}

// NewProtectedTagPost handles creation of a protect tag
func NewProtectedTagPost(ctx *context.Context) {
	if setTagsContext(ctx) != nil {
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplTags)
		return
	}

	repo := ctx.Repo.Repository
	form := web.GetForm(ctx).(*forms.ProtectTagForm)

	pt := &git_model.ProtectedTag{
		RepoID:      repo.ID,
		NamePattern: strings.TrimSpace(form.NamePattern),
	}

	if strings.TrimSpace(form.AllowlistUsers) != "" {
		pt.AllowlistUserIDs, _ = base.StringsToInt64s(strings.Split(form.AllowlistUsers, ","))
	}
	if strings.TrimSpace(form.AllowlistTeams) != "" {
		pt.AllowlistTeamIDs, _ = base.StringsToInt64s(strings.Split(form.AllowlistTeams, ","))
	}

	if err := git_model.InsertProtectedTag(ctx, pt); err != nil {
		ctx.ServerError("InsertProtectedTag", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
	ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
}

// EditProtectedTag render the page to edit a protect tag
func EditProtectedTag(ctx *context.Context) {
	if setTagsContext(ctx) != nil {
		return
	}

	ctx.Data["PageIsEditProtectedTag"] = true

	pt := selectProtectedTagByContext(ctx)
	if pt == nil {
		return
	}

	ctx.Data["name_pattern"] = pt.NamePattern
	ctx.Data["allowlist_users"] = strings.Join(base.Int64sToStrings(pt.AllowlistUserIDs), ",")
	ctx.Data["allowlist_teams"] = strings.Join(base.Int64sToStrings(pt.AllowlistTeamIDs), ",")

	ctx.HTML(http.StatusOK, tplTags)
}

// EditProtectedTagPost handles creation of a protect tag
func EditProtectedTagPost(ctx *context.Context) {
	if setTagsContext(ctx) != nil {
		return
	}

	ctx.Data["PageIsEditProtectedTag"] = true

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplTags)
		return
	}

	pt := selectProtectedTagByContext(ctx)
	if pt == nil {
		return
	}

	form := web.GetForm(ctx).(*forms.ProtectTagForm)

	pt.NamePattern = strings.TrimSpace(form.NamePattern)
	pt.AllowlistUserIDs, _ = base.StringsToInt64s(strings.Split(form.AllowlistUsers, ","))
	pt.AllowlistTeamIDs, _ = base.StringsToInt64s(strings.Split(form.AllowlistTeams, ","))

	if err := git_model.UpdateProtectedTag(ctx, pt); err != nil {
		ctx.ServerError("UpdateProtectedTag", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
	ctx.Redirect(ctx.Repo.Repository.Link() + "/settings/tags")
}

// DeleteProtectedTagPost handles deletion of a protected tag
func DeleteProtectedTagPost(ctx *context.Context) {
	pt := selectProtectedTagByContext(ctx)
	if pt == nil {
		return
	}

	if err := git_model.DeleteProtectedTag(ctx, pt); err != nil {
		ctx.ServerError("DeleteProtectedTag", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
	ctx.Redirect(ctx.Repo.Repository.Link() + "/settings/tags")
}

func setTagsContext(ctx *context.Context) error {
	ctx.Data["Title"] = ctx.Tr("repo.settings.tags")
	ctx.Data["PageIsSettingsTags"] = true

	protectedTags, err := git_model.GetProtectedTags(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetProtectedTags", err)
		return err
	}
	ctx.Data["ProtectedTags"] = protectedTags

	users, err := access_model.GetRepoReaders(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("Repo.Repository.GetReaders", err)
		return err
	}
	ctx.Data["Users"] = users

	if ctx.Repo.Owner.IsOrganization() {
		teams, err := organization.OrgFromUser(ctx.Repo.Owner).TeamsWithAccessToRepo(ctx, ctx.Repo.Repository.ID, perm.AccessModeRead)
		if err != nil {
			ctx.ServerError("Repo.Owner.TeamsWithAccessToRepo", err)
			return err
		}
		ctx.Data["Teams"] = teams
	}

	return nil
}

func selectProtectedTagByContext(ctx *context.Context) *git_model.ProtectedTag {
	id := ctx.FormInt64("id")
	if id == 0 {
		id = ctx.ParamsInt64(":id")
	}

	tag, err := git_model.GetProtectedTagByID(ctx, id)
	if err != nil {
		ctx.ServerError("GetProtectedTagByID", err)
		return nil
	}

	if tag != nil && tag.RepoID == ctx.Repo.Repository.ID {
		return tag
	}

	ctx.NotFound("", fmt.Errorf("ProtectedTag[%v] not associated to repository %v", id, ctx.Repo.Repository))

	return nil
}
