// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"encoding/json"

	"github.com/Unknwon/com"

	api "github.com/gogits/go-gogs-client"

	"github.com/go-gitea/gitea/models"
	"github.com/go-gitea/gitea/modules/context"
	"github.com/go-gitea/gitea/routers/api/v1/convert"
)

// https://github.com/gogits/go-gogs-client/wiki/Repositories#list-hooks
func ListHooks(ctx *context.APIContext) {
	hooks, err := models.GetWebhooksByRepoID(ctx.Repo.Repository.ID)
	if err != nil {
		ctx.Error(500, "GetWebhooksByRepoID", err)
		return
	}

	apiHooks := make([]*api.Hook, len(hooks))
	for i := range hooks {
		apiHooks[i] = convert.ToHook(ctx.Repo.RepoLink, hooks[i])
	}
	ctx.JSON(200, &apiHooks)
}

// https://github.com/gogits/go-gogs-client/wiki/Repositories#create-a-hook
func CreateHook(ctx *context.APIContext, form api.CreateHookOption) {
	if !models.IsValidHookTaskType(form.Type) {
		ctx.Error(422, "", "Invalid hook type")
		return
	}
	for _, name := range []string{"url", "content_type"} {
		if _, ok := form.Config[name]; !ok {
			ctx.Error(422, "", "Missing config option: "+name)
			return
		}
	}
	if !models.IsValidHookContentType(form.Config["content_type"]) {
		ctx.Error(422, "", "Invalid content type")
		return
	}

	if len(form.Events) == 0 {
		form.Events = []string{"push"}
	}
	w := &models.Webhook{
		RepoID:      ctx.Repo.Repository.ID,
		URL:         form.Config["url"],
		ContentType: models.ToHookContentType(form.Config["content_type"]),
		Secret:      form.Config["secret"],
		HookEvent: &models.HookEvent{
			ChooseEvents: true,
			HookEvents: models.HookEvents{
				Create:      com.IsSliceContainsStr(form.Events, string(models.HOOK_EVENT_CREATE)),
				Push:        com.IsSliceContainsStr(form.Events, string(models.HOOK_EVENT_PUSH)),
				PullRequest: com.IsSliceContainsStr(form.Events, string(models.HOOK_EVENT_PULL_REQUEST)),
			},
		},
		IsActive:     form.Active,
		HookTaskType: models.ToHookTaskType(form.Type),
	}
	if w.HookTaskType == models.SLACK {
		channel, ok := form.Config["channel"]
		if !ok {
			ctx.Error(422, "", "Missing config option: channel")
			return
		}
		meta, err := json.Marshal(&models.SlackMeta{
			Channel:  channel,
			Username: form.Config["username"],
			IconURL:  form.Config["icon_url"],
			Color:    form.Config["color"],
		})
		if err != nil {
			ctx.Error(500, "slack: JSON marshal failed", err)
			return
		}
		w.Meta = string(meta)
	}

	if err := w.UpdateEvent(); err != nil {
		ctx.Error(500, "UpdateEvent", err)
		return
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.Error(500, "CreateWebhook", err)
		return
	}

	ctx.JSON(201, convert.ToHook(ctx.Repo.RepoLink, w))
}

// https://github.com/gogits/go-gogs-client/wiki/Repositories#edit-a-hook
func EditHook(ctx *context.APIContext, form api.EditHookOption) {
	w, err := models.GetWebhookByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrWebhookNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetWebhookByID", err)
		}
		return
	}

	if form.Config != nil {
		if url, ok := form.Config["url"]; ok {
			w.URL = url
		}
		if ct, ok := form.Config["content_type"]; ok {
			if !models.IsValidHookContentType(ct) {
				ctx.Error(422, "", "Invalid content type")
				return
			}
			w.ContentType = models.ToHookContentType(ct)
		}

		if w.HookTaskType == models.SLACK {
			if channel, ok := form.Config["channel"]; ok {
				meta, err := json.Marshal(&models.SlackMeta{
					Channel:  channel,
					Username: form.Config["username"],
					IconURL:  form.Config["icon_url"],
					Color:    form.Config["color"],
				})
				if err != nil {
					ctx.Error(500, "slack: JSON marshal failed", err)
					return
				}
				w.Meta = string(meta)
			}
		}
	}

	// Update events
	if len(form.Events) == 0 {
		form.Events = []string{"push"}
	}
	w.PushOnly = false
	w.SendEverything = false
	w.ChooseEvents = true
	w.Create = com.IsSliceContainsStr(form.Events, string(models.HOOK_EVENT_CREATE))
	w.Push = com.IsSliceContainsStr(form.Events, string(models.HOOK_EVENT_PUSH))
	w.PullRequest = com.IsSliceContainsStr(form.Events, string(models.HOOK_EVENT_PULL_REQUEST))
	if err = w.UpdateEvent(); err != nil {
		ctx.Error(500, "UpdateEvent", err)
		return
	}

	if form.Active != nil {
		w.IsActive = *form.Active
	}

	if err := models.UpdateWebhook(w); err != nil {
		ctx.Error(500, "UpdateWebhook", err)
		return
	}

	ctx.JSON(200, convert.ToHook(ctx.Repo.RepoLink, w))
}

func DeleteHook(ctx *context.APIContext) {
	if err := models.DeleteWebhookByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id")); err != nil {
		ctx.Error(500, "DeleteWebhookByRepoID", err)
		return
	}

	ctx.Status(204)
}
