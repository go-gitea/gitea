// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	webhook_service "code.gitea.io/gitea/services/webhook"
)

// swaggeroperation GET /admin/{hookType:default-hooks|system-hooks} admin adminListHooks

// list system or default webhooks
func ListHooks(ctx *context.APIContext) {
	// swagger:operation GET /admin/{hookType} admin adminListHooks
	// ---
	// summary: List system's webhooks
	// produces:
	// - application/json
	// parameters:
	// - name: hookType
	//   in: path
	//   description: whether the hook is system-wide or copied-to-each-new-repo
	//   type: string
	//   enum: [system-hooks, default-hooks]
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/HookList"

	isSystemWebhook := ctx.Params(":hookType") == "system-hooks"

	adminHooks, err := webhook.GetAdminWebhooks(ctx, isSystemWebhook, util.OptionalBoolNone)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetAdminWebhooks", err)
		return
	}

	hooks := make([]*api.Hook, len(adminHooks))
	for i, hook := range adminHooks {
		h, err := webhook_service.ToHook(setting.AppURL+"/admin", hook)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "convert.ToHook", err)
			return
		}
		hooks[i] = h
	}
	ctx.JSON(http.StatusOK, hooks)
}

// get a system/default hook by id
func GetHook(ctx *context.APIContext) {
	// swagger:operation GET /admin/{hookType}/{id} admin adminGetHook
	// ---
	// summary: Get a hook
	// produces:
	// - application/json
	// parameters:
	// - name: hookType
	//   in: path
	//   description: whether the hook is system-wide or copied-to-each-new-repo
	//   type: string
	//   enum: [system-hooks, default-hooks]
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the hook to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Hook"

	isSystemWebhook := ctx.Params(":hookType") == "system-hooks"

	hookID := ctx.ParamsInt64(":id")
	hook, err := webhook.GetAdminWebhook(ctx, hookID, isSystemWebhook)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetAdminWebhook", err)
		}
		return
	}

	h, err := webhook_service.ToHook("/admin/", hook)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convert.ToHook", err)
		return
	}
	ctx.JSON(http.StatusOK, h)
}

// create a system or default hook
func CreateHook(ctx *context.APIContext) {
	// swagger:operation POST /admin/{hookType} admin adminCreateHook
	// ---
	// summary: Create a hook
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: hookType
	//   in: path
	//   description: whether the hook is system-wide or copied-to-each-new-repo
	//   type: string
	//   enum: [system-hooks, default-hooks]
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateHookOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Hook"

	isSystemWebhook := ctx.Params(":hookType") == "system-hooks"

	form := web.GetForm(ctx).(*api.CreateHookOption)

	utils.AddAdminHook(ctx, form, isSystemWebhook)
}

// modify a system or default hook
func EditHook(ctx *context.APIContext) {
	// swagger:operation PATCH /admin/{hookType}/{id} admin adminEditHook
	// ---
	// summary: Update a hook
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: hookType
	//   in: path
	//   description: whether the hook is system-wide or copied-to-each-new-repo
	//   type: string
	//   enum: [system-hooks, default-hooks]
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the hook to update
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditHookOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Hook"

	isSystemWebhook := ctx.Params(":hookType") == "system-hooks"

	form := web.GetForm(ctx).(*api.EditHookOption)

	// TODO in body params
	hookID := ctx.ParamsInt64(":id")
	utils.EditHook(ctx, form, hookID, isSystemWebhook)
}

// delete a system or default hook
func DeleteHook(ctx *context.APIContext) {
	// swagger:operation DELETE /admin/{hookType}/{id} admin adminDeleteHook
	// ---
	// summary: Delete a hook
	// produces:
	// - application/json
	// parameters:
	// - name: hookType
	//   in: path
	//   description: whether the hook is system-wide or copied-to-each-new-repo
	//   type: string
	//   enum: [system-hooks, default-hooks]
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the hook to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	hookID := ctx.ParamsInt64(":id")
	if err := webhook.DeleteAdminWebhook(ctx, hookID); err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteAdminWebhook", err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}
