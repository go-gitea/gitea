// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	pages_model "gitea.dev/models/pages"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/timeutil"
	"gitea.dev/services/context"
	pages_service "gitea.dev/services/pages"
)

const tplPages templates.TplName = "admin/pages"

const pagesOneTimeTokenSessionKey = "admin_pages_one_time_token"

type pagesInstanceView struct {
	ID                    int64
	RootURL               string
	LastCommunicationUnix timeutil.TimeStamp
}

func loadPagesAdminData(ctx *context.Context) bool {
	instances, err := pages_model.ListInstances(ctx)
	if err != nil {
		ctx.ServerError("ListInstances", err)
		return false
	}
	ctx.Data["Title"] = ctx.Tr("admin.pages.title")
	ctx.Data["PageIsAdminPages"] = true
	views := make([]*pagesInstanceView, 0, len(instances))
	for _, instance := range instances {
		views = append(views, &pagesInstanceView{
			ID:                    instance.ID,
			RootURL:               instance.RootURL,
			LastCommunicationUnix: instance.LastCommunicationUnix,
		})
	}
	ctx.Data["PagesInstances"] = views
	ctx.Data["PagesInstanceCount"] = len(instances)
	return true
}

// Pages shows registered Pages instances.
func Pages(ctx *context.Context) {
	if !loadPagesAdminData(ctx) {
		return
	}
	if token := ctx.Session.Get(pagesOneTimeTokenSessionKey); token != nil {
		if err := ctx.Session.Delete(pagesOneTimeTokenSessionKey); err != nil {
			ctx.ServerError("Session.Delete", err)
			return
		}
		if token, ok := token.(string); ok {
			ctx.Data["PagesOneTimeToken"] = token
		}
	}
	ctx.HTML(http.StatusOK, tplPages)
}

// PagesPost manages Pages instances.
func PagesPost(ctx *context.Context) {
	switch ctx.FormString("action") {
	case "create":
		_, token, err := pages_service.CreateInstance(ctx)
		if err != nil {
			ctx.Flash.Error(ctx.Tr("admin.pages.create_error"))
			break
		}
		if err := ctx.Session.Set(pagesOneTimeTokenSessionKey, token); err != nil {
			ctx.ServerError("Session.Set", err)
			return
		}
	case "reset":
		token, err := pages_service.ResetInstanceCredential(ctx, ctx.FormInt64("instance_id"))
		if err != nil {
			ctx.Flash.Error(ctx.Tr("admin.pages.reset_error"))
			break
		}
		if err := ctx.Session.Set(pagesOneTimeTokenSessionKey, token); err != nil {
			ctx.ServerError("Session.Set", err)
			return
		}
	case "remove":
		if err := pages_service.RemoveInstance(ctx, ctx.FormInt64("instance_id")); err != nil {
			ctx.Flash.Error(ctx.Tr("admin.pages.remove_error"))
		} else {
			ctx.Flash.Success(ctx.Tr("admin.pages.remove_success"))
		}
	default:
		ctx.Flash.Error(ctx.Tr("admin.pages.invalid_action"))
	}
	ctx.Redirect(ctx.Link)
}
