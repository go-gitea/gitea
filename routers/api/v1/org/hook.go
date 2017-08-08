// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/v1/convert"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

// ListHooks list an organziation's webhooks
func ListHooks(ctx *context.APIContext) {
	// swagger:route GET /orgs/{orgname}/hooks organization orgListHooks
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: HookList
	//       500: error

	org := ctx.Org.Organization
	orgHooks, err := models.GetWebhooksByOrgID(org.ID)
	if err != nil {
		ctx.Error(500, "GetWebhooksByOrgID", err)
		return
	}
	hooks := make([]*api.Hook, len(orgHooks))
	for i, hook := range orgHooks {
		hooks[i] = convert.ToHook(org.HomeLink(), hook)
	}
	ctx.JSON(200, hooks)
}

// GetHook get an organization's hook by id
func GetHook(ctx *context.APIContext) {
	// swagger:route GET /orgs/{orgname}/hooks/{id} organization orgGetHook
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: Hook
	//       404: notFound
	//       500: error

	org := ctx.Org.Organization
	hookID := ctx.ParamsInt64(":id")
	hook, err := utils.GetOrgHook(ctx, org.ID, hookID)
	if err != nil {
		return
	}
	ctx.JSON(200, convert.ToHook(org.HomeLink(), hook))
}

// CreateHook create a hook for an organization
func CreateHook(ctx *context.APIContext, form api.CreateHookOption) {
	// swagger:route POST /orgs/{orgname}/hooks/ organization orgCreateHook
	//
	//     Consumes:
	//     - application/json
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       201: Hook
	//       422: validationError
	//       500: error

	if !utils.CheckCreateHookOption(ctx, &form) {
		return
	}
	utils.AddOrgHook(ctx, &form)
}

// EditHook modify a hook of a repository
func EditHook(ctx *context.APIContext, form api.EditHookOption) {
	// swagger:route PATCH /orgs/{orgname}/hooks/{id} organization orgEditHook
	//
	//     Consumes:
	//     - application/json
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: Hook
	//       422: validationError
	//       404: notFound
	//       500: error

	hookID := ctx.ParamsInt64(":id")
	utils.EditOrgHook(ctx, &form, hookID)
}

// DeleteHook delete a hook of an organization
func DeleteHook(ctx *context.APIContext) {
	// swagger:route DELETE /orgs/{orgname}/hooks/{id} organization orgDeleteHook
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       204: empty
	//       404: notFound
	//       500: error

	org := ctx.Org.Organization
	hookID := ctx.ParamsInt64(":id")
	if err := models.DeleteWebhookByOrgID(org.ID, hookID); err != nil {
		if models.IsErrWebhookNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "DeleteWebhookByOrgID", err)
		}
		return
	}
	ctx.Status(204)
}
