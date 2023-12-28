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
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models/avatars"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/forms"
	user_service "code.gitea.io/gitea/services/user"
)

const (
	tplSettingsProfile      base.TplName = "user/settings/profile"
	tplSettingsAppearance   base.TplName = "user/settings/appearance"
	tplSettingsOrganization base.TplName = "user/settings/organization"
	tplSettingsRepositories base.TplName = "user/settings/repos"
)

// Profile render user's profile page
func Profile(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.profile")
	ctx.Data["PageIsSettingsProfile"] = true
	ctx.Data["AllowedUserVisibilityModes"] = setting.Service.AllowedUserVisibilityModesSlice.ToVisibleTypeSlice()
	ctx.Data["DisableGravatar"] = setting.Config().Picture.DisableGravatar.Value(ctx)

	ctx.HTML(http.StatusOK, tplSettingsProfile)
}

// HandleUsernameChange handle username changes from user settings and admin interface
func HandleUsernameChange(ctx *context.Context, user *user_model.User, newName string) error {
	oldName := user.Name
	// rename user
	if err := user_service.RenameUser(ctx, user, newName); err != nil {
		switch {
		// Noop as username is not changed
		case user_model.IsErrUsernameNotChanged(err):
			ctx.Flash.Error(ctx.Tr("form.username_has_not_been_changed"))
		// Non-local users are not allowed to change their username.
		case user_model.IsErrUserIsNotLocal(err):
			ctx.Flash.Error(ctx.Tr("form.username_change_not_local_user"))
		case user_model.IsErrUserAlreadyExist(err):
			ctx.Flash.Error(ctx.Tr("form.username_been_taken"))
		case user_model.IsErrEmailAlreadyUsed(err):
			ctx.Flash.Error(ctx.Tr("form.email_been_used"))
		case db.IsErrNameReserved(err):
			ctx.Flash.Error(ctx.Tr("user.form.name_reserved", newName))
		case db.IsErrNamePatternNotAllowed(err):
			ctx.Flash.Error(ctx.Tr("user.form.name_pattern_not_allowed", newName))
		case db.IsErrNameCharsNotAllowed(err):
			ctx.Flash.Error(ctx.Tr("user.form.name_chars_not_allowed", newName))
		default:
			ctx.ServerError("ChangeUserName", err)
		}
		return err
	}
	log.Trace("User name changed: %s -> %s", oldName, newName)
	return nil
}

// ProfilePost response for change user's profile
func ProfilePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.UpdateProfileForm)
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsProfile"] = true
	ctx.Data["AllowedUserVisibilityModes"] = setting.Service.AllowedUserVisibilityModesSlice.ToVisibleTypeSlice()
	ctx.Data["DisableGravatar"] = setting.Config().Picture.DisableGravatar.Value(ctx)

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplSettingsProfile)
		return
	}

	if len(form.Name) != 0 && ctx.Doer.Name != form.Name {
		log.Debug("Changing name for %s to %s", ctx.Doer.Name, form.Name)
		if err := HandleUsernameChange(ctx, ctx.Doer, form.Name); err != nil {
			ctx.Redirect(setting.AppSubURL + "/user/settings")
			return
		}
		ctx.Doer.Name = form.Name
		ctx.Doer.LowerName = strings.ToLower(form.Name)
	}

	ctx.Doer.FullName = form.FullName
	ctx.Doer.KeepEmailPrivate = form.KeepEmailPrivate
	ctx.Doer.Website = form.Website
	ctx.Doer.Location = form.Location
	ctx.Doer.Description = form.Description
	ctx.Doer.KeepActivityPrivate = form.KeepActivityPrivate
	ctx.Doer.Visibility = form.Visibility
	if err := user_model.UpdateUserSetting(ctx, ctx.Doer); err != nil {
		if _, ok := err.(user_model.ErrEmailAlreadyUsed); ok {
			ctx.Flash.Error(ctx.Tr("form.email_been_used"))
			ctx.Redirect(setting.AppSubURL + "/user/settings")
			return
		}
		ctx.ServerError("UpdateUser", err)
		return
	}

	log.Trace("User settings updated: %s", ctx.Doer.Name)
	ctx.Flash.Success(ctx.Tr("settings.update_profile_success"))
	ctx.Redirect(setting.AppSubURL + "/user/settings")
}

// UpdateAvatarSetting update user's avatar
// FIXME: limit size.
func UpdateAvatarSetting(ctx *context.Context, form *forms.AvatarForm, ctxUser *user_model.User) error {
	ctxUser.UseCustomAvatar = form.Source == forms.AvatarLocal
	if len(form.Gravatar) > 0 {
		if form.Avatar != nil {
			ctxUser.Avatar = avatars.HashEmail(form.Gravatar)
		} else {
			ctxUser.Avatar = ""
		}
		ctxUser.AvatarEmail = form.Gravatar
	}

	if form.Avatar != nil && form.Avatar.Filename != "" {
		fr, err := form.Avatar.Open()
		if err != nil {
			return fmt.Errorf("Avatar.Open: %w", err)
		}
		defer fr.Close()

		if form.Avatar.Size > setting.Avatar.MaxFileSize {
			return errors.New(ctx.Tr("settings.uploaded_avatar_is_too_big", form.Avatar.Size/1024, setting.Avatar.MaxFileSize/1024))
		}

		data, err := io.ReadAll(fr)
		if err != nil {
			return fmt.Errorf("io.ReadAll: %w", err)
		}

		st := typesniffer.DetectContentType(data)
		if !(st.IsImage() && !st.IsSvgImage()) {
			return errors.New(ctx.Tr("settings.uploaded_avatar_not_a_image"))
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
		return fmt.Errorf("UpdateUser: %w", err)
	}

	return nil
}

// AvatarPost response for change user's avatar request
func AvatarPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AvatarForm)
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
		UserID:         ctx.Doer.ID,
		IncludePrivate: ctx.IsSigned,
	}

	if opts.Page <= 0 {
		opts.Page = 1
	}

	orgs, total, err := db.FindAndCount[organization.Organization](ctx, opts)
	if err != nil {
		ctx.ServerError("FindOrgs", err)
		return
	}

	ctx.Data["Orgs"] = orgs
	pager := context.NewPagination(int(total), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager
	ctx.HTML(http.StatusOK, tplSettingsOrganization)
}

// Repos display a list of all repositories of the user
func Repos(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.repos")
	ctx.Data["PageIsSettingsRepos"] = true
	ctx.Data["allowAdopt"] = ctx.IsUserSiteAdmin() || setting.Repository.AllowAdoptionOfUnadoptedRepositories
	ctx.Data["allowDelete"] = ctx.IsUserSiteAdmin() || setting.Repository.AllowDeleteOfUnadoptedRepositories

	opts := db.ListOptions{
		PageSize: setting.UI.Admin.UserPagingNum,
		Page:     ctx.FormInt("page"),
	}

	if opts.Page <= 0 {
		opts.Page = 1
	}
	start := (opts.Page - 1) * opts.PageSize
	end := start + opts.PageSize

	adoptOrDelete := ctx.IsUserSiteAdmin() || (setting.Repository.AllowAdoptionOfUnadoptedRepositories && setting.Repository.AllowDeleteOfUnadoptedRepositories)

	ctxUser := ctx.Doer
	count := 0

	if adoptOrDelete {
		repoNames := make([]string, 0, setting.UI.Admin.UserPagingNum)
		repos := map[string]*repo_model.Repository{}
		// We're going to iterate by pagesize.
		root := user_model.UserPath(ctxUser.Name)
		if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if !d.IsDir() || path == root {
				return nil
			}
			name := d.Name()
			if !strings.HasSuffix(name, ".git") {
				return filepath.SkipDir
			}
			name = name[:len(name)-4]
			if repo_model.IsUsableRepoName(name) != nil || strings.ToLower(name) != name {
				return filepath.SkipDir
			}
			if count >= start && count < end {
				repoNames = append(repoNames, name)
			}
			count++
			return filepath.SkipDir
		}); err != nil {
			ctx.ServerError("filepath.WalkDir", err)
			return
		}

		userRepos, _, err := repo_model.GetUserRepositories(ctx, &repo_model.SearchRepoOptions{
			Actor:   ctxUser,
			Private: true,
			ListOptions: db.ListOptions{
				Page:     1,
				PageSize: setting.UI.Admin.UserPagingNum,
			},
			LowerNames: repoNames,
		})
		if err != nil {
			ctx.ServerError("GetUserRepositories", err)
			return
		}
		for _, repo := range userRepos {
			if repo.IsFork {
				if err := repo.GetBaseRepo(ctx); err != nil {
					ctx.ServerError("GetBaseRepo", err)
					return
				}
			}
			repos[repo.LowerName] = repo
		}
		ctx.Data["Dirs"] = repoNames
		ctx.Data["ReposMap"] = repos
	} else {
		repos, count64, err := repo_model.GetUserRepositories(ctx, &repo_model.SearchRepoOptions{Actor: ctxUser, Private: true, ListOptions: opts})
		if err != nil {
			ctx.ServerError("GetUserRepositories", err)
			return
		}
		count = int(count64)

		for i := range repos {
			if repos[i].IsFork {
				if err := repos[i].GetBaseRepo(ctx); err != nil {
					ctx.ServerError("GetBaseRepo", err)
					return
				}
			}
		}

		ctx.Data["Repos"] = repos
	}
	ctx.Data["ContextUser"] = ctxUser
	pager := context.NewPagination(count, opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager
	ctx.HTML(http.StatusOK, tplSettingsRepositories)
}

// Appearance render user's appearance settings
func Appearance(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.appearance")
	ctx.Data["PageIsSettingsAppearance"] = true

	var hiddenCommentTypes *big.Int
	val, err := user_model.GetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyHiddenCommentTypes)
	if err != nil {
		ctx.ServerError("GetUserSetting", err)
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
	form := web.GetForm(ctx).(*forms.UpdateThemeForm)
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsAppearance"] = true

	if ctx.HasError() {
		ctx.Redirect(setting.AppSubURL + "/user/settings/appearance")
		return
	}

	if !form.IsThemeExists() {
		ctx.Flash.Error(ctx.Tr("settings.theme_update_error"))
		ctx.Redirect(setting.AppSubURL + "/user/settings/appearance")
		return
	}

	if err := user_model.UpdateUserTheme(ctx, ctx.Doer, form.Theme); err != nil {
		ctx.Flash.Error(ctx.Tr("settings.theme_update_error"))
		ctx.Redirect(setting.AppSubURL + "/user/settings/appearance")
		return
	}

	log.Trace("Update user theme: %s", ctx.Doer.Name)
	ctx.Flash.Success(ctx.Tr("settings.theme_update_success"))
	ctx.Redirect(setting.AppSubURL + "/user/settings/appearance")
}

// UpdateUserLang update a user's language
func UpdateUserLang(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.UpdateLanguageForm)
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsAppearance"] = true

	if len(form.Language) != 0 {
		if !util.SliceContainsString(setting.Langs, form.Language) {
			ctx.Flash.Error(ctx.Tr("settings.update_language_not_found", form.Language))
			ctx.Redirect(setting.AppSubURL + "/user/settings/appearance")
			return
		}
		ctx.Doer.Language = form.Language
	}

	if err := user_model.UpdateUserSetting(ctx, ctx.Doer); err != nil {
		ctx.ServerError("UpdateUserSetting", err)
		return
	}

	// Update the language to the one we just set
	middleware.SetLocaleCookie(ctx.Resp, ctx.Doer.Language, 0)

	log.Trace("User settings updated: %s", ctx.Doer.Name)
	ctx.Flash.Success(translation.NewLocale(ctx.Doer.Language).Tr("settings.update_language_success"))
	ctx.Redirect(setting.AppSubURL + "/user/settings/appearance")
}

// UpdateUserHiddenComments update a user's shown comment types
func UpdateUserHiddenComments(ctx *context.Context) {
	err := user_model.SetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyHiddenCommentTypes, forms.UserHiddenCommentTypesFromRequest(ctx).String())
	if err != nil {
		ctx.ServerError("SetUserSetting", err)
		return
	}

	log.Trace("User settings updated: %s", ctx.Doer.Name)
	ctx.Flash.Success(ctx.Tr("settings.saved_successfully"))
	ctx.Redirect(setting.AppSubURL + "/user/settings/appearance")
}
