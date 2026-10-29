// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	pages_model "gitea.dev/models/pages"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
	pages_service "gitea.dev/services/pages"
)

const tplPages templates.TplName = "admin/pages"

func loadPagesAdminData(ctx *context.Context) bool {
	instances, err := pages_model.ListInstances(ctx)
	if err != nil {
		ctx.ServerError("ListInstances", err)
		return false
	}
	policyEnabled, _, err := pages_service.GetTXTVerificationPolicy(ctx)
	if err != nil {
		ctx.ServerError("GetPagesTXTPolicy", err)
		return false
	}
	ctx.Data["Title"] = ctx.Tr("admin.pages.title")
	ctx.Data["PageIsAdminPages"] = true
	ctx.Data["PagesInstances"] = instances
	ctx.Data["PagesTXTPolicyEnabled"] = policyEnabled
	return true
}

// Pages shows registered Pages instances and the global TXT verification policy.
func Pages(ctx *context.Context) {
	if !loadPagesAdminData(ctx) {
		return
	}
	ctx.HTML(http.StatusOK, tplPages)
}

// PagesPost manages Pages instances and the global TXT verification policy.
func PagesPost(ctx *context.Context) {
	switch ctx.FormString("action") {
	case "create":
		_, token, err := pages_service.CreateInstance(ctx)
		if err != nil {
			ctx.Flash.Error(ctx.Tr("admin.pages.create_error"))
			ctx.Redirect(ctx.Link)
			return
		}
		if !loadPagesAdminData(ctx) {
			return
		}
		ctx.Data["PagesOneTimeToken"] = token
		ctx.HTML(http.StatusOK, tplPages)
	case "reset":
		token, err := pages_service.ResetInstanceCredential(ctx, ctx.FormInt64("instance_id"))
		if err != nil {
			ctx.Flash.Error(ctx.Tr("admin.pages.reset_error"))
			ctx.Redirect(ctx.Link)
			return
		}
		if !loadPagesAdminData(ctx) {
			return
		}
		ctx.Data["PagesOneTimeToken"] = token
		ctx.HTML(http.StatusOK, tplPages)
	case "remove":
		if err := pages_service.RemoveInstance(ctx, ctx.FormInt64("instance_id")); err != nil {
			ctx.Flash.Error(ctx.Tr("admin.pages.remove_error"))
		} else {
			ctx.Flash.Success(ctx.Tr("admin.pages.remove_success"))
		}
		ctx.Redirect(ctx.Link)
	case "policy":
		if err := pages_service.SetTXTVerificationPolicy(ctx, ctx.FormBool("enabled")); err != nil {
			ctx.Flash.Error(ctx.Tr("admin.pages.policy_error"))
		} else {
			ctx.Flash.Success(ctx.Tr("admin.pages.policy_success"))
		}
		ctx.Redirect(ctx.Link)
	default:
		ctx.Flash.Error(ctx.Tr("admin.pages.invalid_action"))
		ctx.Redirect(ctx.Link)
	}
}
