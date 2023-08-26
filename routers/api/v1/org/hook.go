// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/param"
	webhook_service "code.gitea.io/gitea/services/webhook"
)

// ListHooks list an organziation's webhooks
func ListHooks(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/hooks organization orgListHooks
	// ---
	// summary: List an organization's webhooks
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
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

	webhook_service.ListOwnerHooks(
		ctx,
		param.GetListOptions(ctx),
		ctx.ContextUser,
	)
}

// GetHook get an organization's hook by id
func GetHook(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/hooks/{id} organization orgGetHook
	// ---
	// summary: Get a hook
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
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

	hook, err := webhook_service.GetOwnerHook(ctx.ContextUser.ID, ctx.ParamsInt64("id"))
	if err != nil {
		return
	}

	apiHook, err := webhook_service.ToHook(ctx.ContextUser.HomeLink(), hook)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusOK, apiHook)
}

// CreateHook create a hook for an organization
func CreateHook(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/hooks organization orgCreateHook
	// ---
	// summary: Create a hook
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateHookOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Hook"

	webhook_service.AddOwnerHook(
		ctx,
		ctx.ContextUser,
		web.GetForm(ctx).(*api.CreateHookOption),
	)
}

// EditHook modify a hook of an organization
func EditHook(ctx *context.APIContext) {
	// swagger:operation PATCH /orgs/{org}/hooks/{id} organization orgEditHook
	// ---
	// summary: Update a hook
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
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

	webhook, status, logTitle, err := webhook_service.EditOwnerHook(
		ctx,
		ctx.ContextUser,
		web.GetForm(ctx).(*api.EditHookOption),
		ctx.ParamsInt64("id"),
	)
	if err != nil {
		ctx.Error(status, logTitle, err)
		return
	}

	ctx.JSON(http.StatusCreated, webhook)
}

// DeleteHook delete a hook of an organization
func DeleteHook(ctx *context.APIContext) {
	// swagger:operation DELETE /orgs/{org}/hooks/{id} organization orgDeleteHook
	// ---
	// summary: Delete a hook
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
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

	hookID := ctx.ParamsInt64("id")
	err := webhook_model.DeleteWebhookByOwnerID(
		ctx.ContextUser.ID,
		hookID,
	)
	if err != nil {
		if webhook_model.IsErrWebhookNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteWebhookByOwnerID", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}
