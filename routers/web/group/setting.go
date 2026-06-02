// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"gitea.dev/models/db"
	group_model "gitea.dev/models/group"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/typesniffer"
	"gitea.dev/modules/web"
	shared_group "gitea.dev/routers/web/shared/group"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	group_service "gitea.dev/services/group"
	repo_service "gitea.dev/services/repository"
	user_service "gitea.dev/services/user"
)

const (
	tplSettingsOptions templates.TplName = "group/settings/options"
)

func RedirectToDefaultSetting(ctx *context.Context) {
	ctx.Redirect(ctx.RepoGroup.OrgGroupLink + "/settings/actions/runners")
}

// Settings render the main settings page
func Settings(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("group.settings")
	ctx.Data["PageIsGroupSettings"] = true
	ctx.Data["PageIsSettingsOptions"] = true
	ctx.Data["CurrentVisibility"] = ctx.RepoGroup.Group.Visibility
	ctx.Data["ContextUser"] = ctx.ContextUser

	err := shared_group.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}
	if err = ctx.RepoGroup.Group.LoadParentGroup(ctx); err != nil {
		ctx.ServerError("LoadParentGroup", err)
		return
	}
	opts := group_model.FindGroupsOptions{
		ActorID: ctx.Doer.ID,
		OwnerID: ctx.RepoGroup.Group.OwnerID,
	}
	cond := group_model.AccessibleGroupCondition(ctx.Doer)
	cond = cond.And(opts.ToConds())
	groups, err := group_model.FindGroupsByCond(ctx, &group_model.FindGroupsOptions{
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		ParentGroupID: -1,
	}, cond)
	if err != nil {
		ctx.ServerError("FindGroupsByCond", err)
		return
	}
	for _, g := range groups {
		err = g.LoadAccessibleSubgroups(ctx, true, ctx.Doer, false)
		if err != nil {
			ctx.ServerError("LoadAccessibleSubgroups", err)
			return
		}
	}

	ctx.Data["Groups"] = groups

	ctx.HTML(http.StatusOK, tplSettingsOptions)
}

func SettingsPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.UpdateGroupSettingForm)
	ctx.Data["Title"] = ctx.Tr("group.settings")
	ctx.Data["PageIsGroupSettings"] = true
	ctx.Data["PageIsSettingsOptions"] = true

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplSettingsOptions)
		return
	}
	group := ctx.RepoGroup.Group

	hasCycle, err := group_model.CheckCycle(ctx, group.ID, form.ParentGroupID)
	if err != nil {
		ctx.ServerError("CheckCycle", err)
		return
	}
	if hasCycle {
		ctx.Flash.Error(ctx.Tr("group.settings.cycle_detected"))
		ctx.Redirect(ctx.RepoGroup.OrgGroupLink + "/settings")
		return
	}

	opts := &group_service.UpdateOptions{
		Description: optional.Some(form.Description),
		Visibility:  optional.Some(form.Visibility),
	}
	if form.Name != group.Name {
		opts.Name = optional.Some(form.Name)
	}
	visibilityChanged := group.Visibility != form.Visibility
	if err := group_service.UpdateGroup(ctx, group, opts); err != nil {
		ctx.ServerError("UpdateGroup", err)
		return
	}
	if visibilityChanged {
		repos, _, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
			Actor:   ctx.ContextUser,
			Private: true,
			GroupID: group.ID,
		})
		if err != nil {
			ctx.ServerError("SearchRepositories", err)
			return
		}
		for _, repo := range repos {
			if err = repo_service.UpdateRepository(ctx, repo, true); err != nil {
				ctx.ServerError("UpdateRepository", err)
				return
			}
		}
	}
	log.Trace("Group setting updated: '%s'", group.Name)
	ctx.Flash.Success(ctx.Tr("group.settings.update_setting_success"))
	ctx.Redirect(ctx.RepoGroup.OrgGroupLink + "/settings")
}

// SettingsAvatar response for change avatar on settings page
func SettingsAvatar(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AvatarForm)
	form.Source = forms.AvatarLocal
	if err := updateAvatarSetting(ctx, form, ctx.RepoGroup.Group); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("group.settings.update_avatar_success"))
	}

	ctx.Redirect(ctx.RepoGroup.OrgGroupLink + "/settings")
}

// SettingsDeleteAvatar response for delete avatar on settings page
func SettingsDeleteAvatar(ctx *context.Context) {
	u := ctx.ContextUser

	if ctx.Org.Organization != nil {
		u = ctx.Org.Organization.AsUser()
	}

	if err := user_service.DeleteAvatar(ctx, u); err != nil {
		ctx.Flash.Error(err.Error())
	}

	ctx.JSONRedirect(ctx.RepoGroup.OrgGroupLink + "/settings")
}

func updateAvatarSetting(ctx *context.Context, form *forms.AvatarForm, group *group_model.Group) error {
	if form.Avatar != nil && form.Avatar.Filename != "" {
		fr, err := form.Avatar.Open()
		if err != nil {
			return fmt.Errorf("Avatar.Open: %w", err)
		}
		defer fr.Close()

		if form.Avatar.Size > setting.Avatar.MaxFileSize {
			return errors.New(ctx.Locale.TrString("settings.uploaded_avatar_is_too_big", form.Avatar.Size/1024, setting.Avatar.MaxFileSize/1024))
		}

		data, err := io.ReadAll(fr)
		if err != nil {
			return fmt.Errorf("io.ReadAll: %w", err)
		}

		st := typesniffer.DetectContentType(data)
		if !(st.IsImage() && !st.IsSvgImage()) {
			return errors.New(ctx.Locale.TrString("settings.uploaded_avatar_not_a_image"))
		}
		if err = group_service.UploadAvatar(ctx, group, data); err != nil {
			return fmt.Errorf("UploadAvatar: %w", err)
		}
	}
	return nil
}
