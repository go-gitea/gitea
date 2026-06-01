// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	user_service "gitea.dev/services/user"
)

const (
	tplSettingsSavedReplies templates.TplName = "user/settings/saved_replies"
)

func SavedReplies(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.saved_replies")
	ctx.Data["PageIsSettingsSavedReplies"] = true

	savedReplies, err := user_model.GetUserSavedReplies(ctx, ctx.Doer.ID, "")
	if err != nil {
		ctx.ServerError("GetUserSavedReplies", err)
		return
	}
	ctx.Data["UserSavedReplies"] = savedReplies
	ctx.HTML(http.StatusOK, tplSettingsSavedReplies)
}

func SavedRepliesJSON(ctx *context.Context) {
	savedReplies, err := user_model.GetUserSavedReplies(ctx, ctx.Doer.ID, "")
	if err != nil {
		ctx.ServerError("GetUserSavedReplies", err)
		return
	}
	type savedReplyJSON struct {
		ID      int64  `json:"id"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	result := make([]savedReplyJSON, 0, len(savedReplies))
	for _, sr := range savedReplies {
		result = append(result, savedReplyJSON{
			ID:      sr.ID,
			Title:   sr.Title,
			Content: sr.Content,
		})
	}
	ctx.JSON(http.StatusOK, result)
}

func savedRepliesPost(ctx *context.Context, form *forms.SavedReplyForm, user *user_model.User) error {
	switch form.Action {
	case "create":
		err := user_service.CreateSavedReply(ctx, user, form.Title, form.Content)
		if errors.Is(err, util.ErrInvalidArgument) {
			return util.ErrorWrapTranslatable(err, "settings.saved_replies.create.failure", err.Error())
		}
		return err
	case "edit":
		err := user_service.UpdateSavedReply(ctx, user, form.ID, form.Title, form.Content)
		if errors.Is(err, util.ErrInvalidArgument) {
			return util.ErrorWrapTranslatable(err, "settings.saved_replies.edit.failure", err.Error())
		}
		return err
	case "delete":
		err := user_service.DeleteSavedReply(ctx, user, form.ID)
		if errors.Is(err, util.ErrInvalidArgument) {
			return util.ErrorWrapTranslatable(err, "settings.saved_replies.delete.failure", err.Error())
		}
		return err
	}
	setting.PanicInDevOrTesting("Unknown action: %q", form.Action)
	return errors.New("unknown action")
}

func SavedRepliesPost(ctx *context.Context) {
	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	form := web.GetForm(ctx).(*forms.SavedReplyForm)
	err := savedRepliesPost(ctx, form, ctx.Doer)
	if err == nil {
		redirect := setting.AppSubURL + "/user/settings/saved_replies"
		ctx.JSONRedirect(redirect)
	} else if errTr := util.ErrorAsTranslatable(err); errTr != nil {
		ctx.JSONError(errTr.Translate(ctx.Locale))
	} else if errors.Is(err, util.ErrNotExist) {
		ctx.JSONError(ctx.Locale.Tr("error.not_found"))
	} else {
		ctx.ServerError("SavedRepliesPost", err)
	}
}
