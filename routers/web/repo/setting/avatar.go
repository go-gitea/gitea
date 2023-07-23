// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"io"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	repo_service "code.gitea.io/gitea/services/repository"
)

// UpdateAvatarSetting update repo's avatar
func UpdateAvatarSetting(ctx *context.Context, form forms.AvatarForm) error {
	ctxRepo := ctx.Repo.Repository

	if form.Avatar == nil {
		// No avatar is uploaded and we not removing it here.
		// No random avatar generated here.
		// Just exit, no action.
		if ctxRepo.CustomAvatarRelativePath() == "" {
			log.Trace("No avatar was uploaded for repo: %d. Default icon will appear instead.", ctxRepo.ID)
		}
		return nil
	}

	r, err := form.Avatar.Open()
	if err != nil {
		return fmt.Errorf("Avatar.Open: %w", err)
	}
	defer r.Close()

	if form.Avatar.Size > setting.Avatar.MaxFileSize {
		return errors.New(ctx.Tr("settings.uploaded_avatar_is_too_big"))
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("io.ReadAll: %w", err)
	}
	st := typesniffer.DetectContentType(data)
	if !(st.IsImage() && !st.IsSvgImage()) {
		return errors.New(ctx.Tr("settings.uploaded_avatar_not_a_image"))
	}
	if err = repo_service.UploadAvatar(ctx, ctxRepo, data); err != nil {
		return fmt.Errorf("UploadAvatar: %w", err)
	}
	return nil
}

// SettingsAvatar save new POSTed repository avatar
func SettingsAvatar(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AvatarForm)
	form.Source = forms.AvatarLocal
	if err := UpdateAvatarSetting(ctx, *form); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.update_avatar_success"))
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings")
}

// SettingsDeleteAvatar delete repository avatar
func SettingsDeleteAvatar(ctx *context.Context) {
	if err := repo_service.DeleteAvatar(ctx, ctx.Repo.Repository); err != nil {
		ctx.Flash.Error(fmt.Sprintf("DeleteAvatar: %v", err))
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings")
}
