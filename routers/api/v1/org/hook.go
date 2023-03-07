// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
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

	opts := &webhook_model.ListWebhookOptions{
		ListOptions: utils.GetListOptions(ctx),
		OrgID:       ctx.Org.Organization.ID,
	}

	count, err := webhook_model.CountWebhooksByOpts(opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	orgHooks, err := webhook_model.ListWebhooksByOpts(ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	hooks := make([]*api.Hook, len(orgHooks))
	for i, hook := range orgHooks {
		hooks[i], err = webhook_service.ToHook(ctx.Org.Organization.AsUser().HomeLink(), hook)
		if err != nil {
			ctx.InternalServerError(err)
			return
		}
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, hooks)
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

	org := ctx.Org.Organization
	hookID := ctx.ParamsInt64(":id")
	hook, err := utils.GetOrgHook(ctx, org.ID, hookID)
	if err != nil {
		return
	}

	apiHook, err := webhook_service.ToHook(org.AsUser().HomeLink(), hook)
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

	form := web.GetForm(ctx).(*api.CreateHookOption)
	// TODO in body params
	if !utils.CheckCreateHookOption(ctx, form) {
		return
	}
	utils.AddOrgHook(ctx, form)
}

// EditHook modify a hook of a repository
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

	form := web.GetForm(ctx).(*api.EditHookOption)

	// TODO in body params
	hookID := ctx.ParamsInt64(":id")
	utils.EditOrgHook(ctx, form, hookID)
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

	org := ctx.Org.Organization
	hookID := ctx.ParamsInt64(":id")
	if err := webhook_model.DeleteWebhookByOrgID(org.ID, hookID); err != nil {
		if webhook_model.IsErrWebhookNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteWebhookByOrgID", err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}
