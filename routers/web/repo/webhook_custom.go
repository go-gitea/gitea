// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"io"
	"net/http"

	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/web"
	cwebhook "code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/services/forms"
	webhook_service "code.gitea.io/gitea/services/webhook"
)

// CustomHooksNewPost response for creating custom hook
func CustomHooksNewPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewCustomWebhookForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook.HookEvent{}}
	ctx.Data["HookType"] = webhook.CUSTOM

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}
	ctx.Data["BaseLink"] = orCtx.LinkNew

	hookType, isCustom := checkHookType(ctx)
	if !isCustom {
		ctx.NotFound("checkHookType", nil)
		return
	}
	hook := cwebhook.Webhooks[hookType]
	if isCustom {
		ctx.Data["CustomHook"] = hook
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, orCtx.NewTemplate)
		return
	}

	meta, err := json.Marshal(&webhook_service.CustomMeta{
		DisplayName: form.DisplayName,
		Form:        form.Form,
		Secret:      form.Secret,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w := &webhook.Webhook{
		URL:             form.DisplayName,
		Secret:          form.Secret,
		RepoID:          orCtx.RepoID,
		ContentType:     webhook.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		Type:            webhook.CUSTOM,
		CustomID:        hook.ID,
		Meta:            string(meta),
		OrgID:           orCtx.OrgID,
		IsSystemWebhook: orCtx.IsSystemWebhook,
	}
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := webhook.CreateWebhook(ctx, w); err != nil {
		ctx.ServerError("CreateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_hook_success"))
	ctx.Redirect(orCtx.Link)
}

// CustomHooksEditPost response for editing custom hook
func CustomHooksEditPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewCustomWebhookForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, orCtx.NewTemplate)
		return
	}

	meta, err := json.Marshal(&webhook_service.CustomMeta{
		DisplayName: form.DisplayName,
		Form:        form.Form,
		Secret:      form.Secret,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w.Meta = string(meta)
	w.URL = form.DisplayName
	w.Secret = form.Secret
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := webhook.UpdateWebhook(w); err != nil {
		ctx.ServerError("UpdateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// CustomWebhookImage gets the image for a custom webhook
func CustomWebhookImage(ctx *context.Context) {
	id := ctx.Params("custom_id")
	hook, ok := cwebhook.Webhooks[id]
	if !ok {
		ctx.NotFound("no webhook found", nil)
		return
	}
	img, err := hook.Image()
	if err != nil {
		ctx.ServerError(fmt.Sprintf("webhook image not found for %q", id), err)
		return
	}
	defer img.Close()
	if _, err := io.Copy(ctx.Resp, img); err != nil {
		ctx.ServerError(fmt.Sprintf("could not stream webhook image for %q", id), err)
		return
	}
}
