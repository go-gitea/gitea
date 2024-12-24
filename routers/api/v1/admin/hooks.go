// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	webhook_service "code.gitea.io/gitea/services/webhook"
)

// ListHooks list system's webhooks
func ListHooks(ctx *context.APIContext) {
	// swagger:operation GET /admin/hooks admin adminListHooks
	// ---
	// summary: List system's webhooks
	// produces:
	// - application/json
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/HookList"

	sysHooks, err := webhook.GetSystemWebhooks(ctx, optional.None[bool]())
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetSystemWebhooks", err)
		return
	}
	hooks := make([]*api.Hook, len(sysHooks))
	for i, hook := range sysHooks {
		h, err := webhook_service.ToHook(setting.AppURL+"/-/admin", hook)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "convert.ToHook", err)
			return
		}
		hooks[i] = h
	}
	ctx.JSON(http.StatusOK, hooks)
}

// GetHook get an organization's hook by id
func GetHook(ctx *context.APIContext) {
	// swagger:operation GET /admin/hooks/{id} admin adminGetHook
	// ---
	// summary: Get a hook
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the hook to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Hook"

	hookID := ctx.PathParamInt64("id")
	hook, err := webhook.GetSystemOrDefaultWebhook(ctx, hookID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetSystemOrDefaultWebhook", err)
		}
		return
	}
	h, err := webhook_service.ToHook("/-/admin/", hook)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convert.ToHook", err)
		return
	}
	ctx.JSON(http.StatusOK, h)
}

// CreateHook create a hook for an organization
func CreateHook(ctx *context.APIContext) {
	// swagger:operation POST /admin/hooks admin adminCreateHook
	// ---
	// summary: Create a hook
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateHookOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Hook"

	form := web.GetForm(ctx).(*api.CreateHookOption)

	utils.AddSystemHook(ctx, form)
}

// EditHook modify a hook of a repository
func EditHook(ctx *context.APIContext) {
	// swagger:operation PATCH /admin/hooks/{id} admin adminEditHook
	// ---
	// summary: Update a hook
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
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

	form := web.GetForm(ctx).(*api.EditHookOption)

	// TODO in body params
	hookID := ctx.PathParamInt64("id")
	utils.EditSystemHook(ctx, form, hookID)
}

// DeleteHook delete a system hook
func DeleteHook(ctx *context.APIContext) {
	// swagger:operation DELETE /admin/hooks/{id} admin adminDeleteHook
	// ---
	// summary: Delete a hook
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the hook to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	hookID := ctx.PathParamInt64("id")
	if err := webhook.DeleteDefaultSystemWebhook(ctx, hookID); err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteDefaultSystemWebhook", err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}
