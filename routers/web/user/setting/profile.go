// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/translation"
	"gitea.dev/modules/typesniffer"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/modules/web/middleware"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	user_service "gitea.dev/services/user"
	"gitea.dev/services/webtheme"
)

const (
	tplSettingsProfile      templates.TplName = "user/settings/profile"
	tplSettingsAppearance   templates.TplName = "user/settings/appearance"
	tplSettingsOrganization templates.TplName = "user/settings/organization"
	tplSettingsRepositories templates.TplName = "user/settings/repos"
)

// Profile render user's profile page
func Profile(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.profile")
	ctx.Data["PageIsSettingsProfile"] = true
	ctx.Data["AllowedUserVisibilityModes"] = setting.Service.AllowedUserVisibilityModesSlice.ToVisibleTypeSlice()
	ctx.Data["DisableGravatar"] = setting.Config().Picture.DisableGravatar.Value(ctx)
	ctx.Data["ForceUserActivityVisibility"] = setting.Service.ForceUserActivityVisibility

	ctx.HTML(http.StatusOK, tplSettingsProfile)
}

// ProfilePost response for change user's profile
func ProfilePost(ctx *context.Context) {
	form := context.GetFetchActionForm[*forms.UpdateProfileForm](ctx)
	if form == nil {
		return
	}

	if form.Name != "" {
		if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureChangeUsername) {
			ctx.JSONErrorWithField(ctx.Tr("form.change_username_disabled"), "Name")
			return
		}
		if err := user_service.RenameUser(ctx, ctx.Doer, form.Name, ctx.Doer); err != nil {
			switch {
			case user_model.IsErrUserIsNotLocal(err):
				ctx.JSONErrorWithField(ctx.Tr("form.username_change_not_local_user"), "Name")
			case user_model.IsErrUserAlreadyExist(err):
				ctx.JSONErrorWithField(ctx.Tr("form.username_been_taken"), "Name")
			case db.IsErrNameReserved(err):
				ctx.JSONErrorWithField(ctx.Tr("user.form.name_reserved", form.Name), "Name")
			case db.IsErrNamePatternNotAllowed(err):
				ctx.JSONErrorWithField(ctx.Tr("user.form.name_pattern_not_allowed", form.Name), "Name")
			case db.IsErrNameCharsNotAllowed(err):
				ctx.JSONErrorWithField(ctx.Tr("user.form.name_chars_not_allowed", form.Name), "Name")
			default:
				ctx.JSONErrorAuto(err)
			}
			return
		}
	}

	if form.FullName != nil && user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureChangeFullName) {
		ctx.JSONErrorWithField(ctx.Tr("form.change_full_name_disabled"), "FullName")
		return
	}

	if err := user_service.UpdateUser(ctx, ctx.Doer, &user_service.UpdateOptions{
		KeepEmailPrivate:    optional.Some(form.KeepEmailPrivate),
		FullName:            optional.FromPtr(form.FullName),
		Description:         optional.Some(form.Description),
		Website:             optional.Some(form.Website),
		Location:            optional.Some(form.Location),
		Visibility:          optional.Some(form.Visibility),
		KeepActivityPrivate: util.Iif(setting.Service.ForceUserActivityVisibility == "", optional.Some(form.KeepActivityPrivate), nil),
	}); err != nil {
		ctx.JSONErrorAuto(err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.update_profile_success"))
	ctx.JSONRedirect(setting.AppSubURL + "/user/settings")
}

// UpdateAvatarSetting update user's avatar
// FIXME: limit size.
func UpdateAvatarSetting(ctx *context.Context, form *forms.AvatarForm, ctxUser *user_model.User) error {
	ctxUser.UseCustomAvatar = form.Source == forms.AvatarLocal
	if len(form.Gravatar) > 0 {
		ctxUser.Avatar = "" // UploadAvatar sets the real storage path when a file is uploaded
		ctxUser.AvatarEmail = form.Gravatar
	}

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
		if err = user_service.UploadAvatar(ctx, ctxUser, data); err != nil {
			return fmt.Errorf("UploadAvatar: %w", err)
		}
	} else if ctxUser.UseCustomAvatar && ctxUser.Avatar == "" {
		// No avatar is uploaded but setting has been changed to enable,
		// generate a random one when needed.
		if err := user_model.GenerateRandomAvatar(ctx, ctxUser); err != nil {
			log.Error("GenerateRandomAvatar[%d]: %v", ctxUser.ID, err)
		}
	}

	if err := user_model.UpdateUserCols(ctx, ctxUser, "avatar", "avatar_email", "use_custom_avatar"); err != nil {
		return fmt.Errorf("UpdateUserCols: %w", err)
	}

	return nil
}

// AvatarPost response for change user's avatar request
func AvatarPost(ctx *context.Context) {
	form := web.GetForm[*forms.AvatarForm](ctx)
	if err := UpdateAvatarSetting(ctx, form, ctx.Doer); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("settings.update_avatar_success"))
	}

	ctx.Redirect(setting.AppSubURL + "/user/settings")
}

// DeleteAvatar render delete avatar page
func DeleteAvatar(ctx *context.Context) {
	if err := user_service.DeleteAvatar(ctx, ctx.Doer); err != nil {
		ctx.Flash.Error(err.Error())
	}

	ctx.JSONRedirect(setting.AppSubURL + "/user/settings")
}

// Organization render all the organization of the user
func Organization(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.organization")
	ctx.Data["PageIsSettingsOrganization"] = true

	opts := organization.FindOrgOptions{
		ListOptions: db.ListOptions{
			PageSize: setting.UI.Admin.UserPagingNum,
			Page:     ctx.FormInt("page"),
		},
		UserID:            ctx.Doer.ID,
		IncludeVisibility: structs.VisibleTypePrivate,
	}

	if opts.Page <= 0 {
		opts.Page = 1
	}

	orgs, total, err := db.FindAndCount[organization.Organization](ctx, opts)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	ctx.Data["Orgs"] = orgs
	pager := context.NewPagerBuilder(ctx).TotalCount(total).PerPageLimit(opts.PageSize).CurPage(opts.Page).Build()
	ctx.Data["Page"] = pager
	ctx.HTML(http.StatusOK, tplSettingsOrganization)
}

// Repos display a list of all repositories of the user
func Repos(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.repos")
	ctx.Data["PageIsSettingsRepos"] = true
	opts := db.ListOptions{
		PageSize: setting.UI.Admin.UserPagingNum,
		Page:     ctx.FormInt("page"),
	}

	if opts.Page <= 0 {
		opts.Page = 1
	}
	repos, count, err := repo_model.GetUserRepositories(ctx, repo_model.SearchRepoOptions{OwnerID: ctx.Doer.ID, Private: true, ListOptions: opts})
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	for i := range repos {
		if repos[i].IsFork {
			if err := repos[i].GetBaseRepo(ctx); err != nil {
				ctx.ServerErrorAuto(err)
				return
			}
		}
	}

	ctx.Data["Repos"] = repos
	pager := context.NewPagerBuilder(ctx).TotalCount(count).PerPageLimit(opts.PageSize).CurPage(opts.Page).Build()
	ctx.Data["Page"] = pager
	ctx.HTML(http.StatusOK, tplSettingsRepositories)
}

// Appearance render user's appearance settings
func Appearance(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.appearance")
	ctx.Data["PageIsSettingsAppearance"] = true
	ctx.Data["AllThemes"] = webtheme.GetAvailableThemes()

	var hiddenCommentTypes *big.Int
	val, err := user_model.GetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyHiddenCommentTypes)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	hiddenCommentTypes, _ = new(big.Int).SetString(val, 10) // we can safely ignore the failed conversion here

	ctx.Data["IsCommentTypeGroupChecked"] = func(commentTypeGroup string) bool {
		return forms.IsUserHiddenCommentTypeGroupChecked(commentTypeGroup, hiddenCommentTypes)
	}

	ctx.HTML(http.StatusOK, tplSettingsAppearance)
}

// UpdateUIThemePost is used to update users' specific theme
func UpdateUIThemePost(ctx *context.Context) {
	form := web.GetForm[*forms.UpdateThemeForm](ctx)
	ctx.Data["Title"] = ctx.Tr("settings_title")
	ctx.Data["PageIsSettingsAppearance"] = true

	if ctx.HasError() {
		ctx.Flash.Error(ctx.GetErrMsg())
		ctx.Redirect(setting.AppSubURL + "/user/settings/appearance")
		return
	}

	if webtheme.GetThemeMetaInfo(form.Theme) == nil {
		ctx.Flash.Error(ctx.Tr("settings.theme_update_error"))
		ctx.Redirect(setting.AppSubURL + "/user/settings/appearance")
		return
	}

	opts := &user_service.UpdateOptions{
		Theme: optional.Some(form.Theme),
	}
	if err := user_service.UpdateUser(ctx, ctx.Doer, opts); err != nil {
		ctx.Flash.Error(ctx.Tr("settings.theme_update_error"))
	} else {
		ctx.Flash.Success(ctx.Tr("settings.theme_update_success"))
	}

	ctx.Redirect(setting.AppSubURL + "/user/settings/appearance")
}

// UpdateUserLang update a user's language
func UpdateUserLang(ctx *context.Context) {
	form := web.GetForm[*forms.UpdateLanguageForm](ctx)
	ctx.Data["Title"] = ctx.Tr("settings_title")
	ctx.Data["PageIsSettingsAppearance"] = true

	if form.Language != "" {
		if !util.SliceContainsString(setting.Langs, form.Language) {
			ctx.Flash.Error(ctx.Tr("settings.update_language_not_found", form.Language))
			ctx.Redirect(setting.AppSubURL + "/user/settings/appearance")
			return
		}
	}

	opts := &user_service.UpdateOptions{
		Language: optional.Some(form.Language),
	}
	if err := user_service.UpdateUser(ctx, ctx.Doer, opts); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	// Update the language to the one we just set
	middleware.SetLocaleCookie(ctx.Resp, ctx.Doer.Language, 0)

	log.Trace("User settings updated: %s", ctx.Doer.Name)
	ctx.Flash.Success(translation.NewLocale(ctx.Doer.Language).TrString("settings.update_language_success"))
	ctx.Redirect(setting.AppSubURL + "/user/settings/appearance")
}

// UpdateUserHiddenComments update a user's shown comment types
func UpdateUserHiddenComments(ctx *context.Context) {
	err := user_model.SetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyHiddenCommentTypes, forms.UserHiddenCommentTypesFromRequest(ctx).String())
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	log.Trace("User settings updated: %s", ctx.Doer.Name)
	ctx.Flash.Success(ctx.Tr("settings.saved_successfully"))
	ctx.Redirect(setting.AppSubURL + "/user/settings/appearance")
}
