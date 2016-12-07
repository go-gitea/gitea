// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/v1/convert"
	"encoding/json"
	"github.com/Unknwon/com"
)

// GetOrgHook get an organization's webhook. If there is an error, write to
// `ctx` accordingly and return the error
func GetOrgHook(ctx *context.APIContext, orgID, hookID int64) (*models.Webhook, error) {
	w, err := models.GetWebhookByOrgID(orgID, hookID)
	if err != nil {
		if models.IsErrWebhookNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetWebhookByOrgID", err)
		}
		return nil, err
	}
	return w, nil
}

// GetRepoHook get a repo's webhook. If there is an error, write to `ctx`
// accordingly and return the error
func GetRepoHook(ctx *context.APIContext, repoID, hookID int64) (*models.Webhook, error) {
	w, err := models.GetWebhookByRepoID(repoID, hookID)
	if err != nil {
		if models.IsErrWebhookNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetWebhookByID", err)
		}
		return nil, err
	}
	return w, nil
}

// CheckCreateHookOption check if a CreateHookOption form is valid. If invalid,
// write the appropriate error to `ctx`. Return whether the form is valid
func CheckCreateHookOption(ctx *context.APIContext, form *api.CreateHookOption) bool {
	if !models.IsValidHookTaskType(form.Type) {
		ctx.Error(422, "", "Invalid hook type")
		return false
	}
	for _, name := range []string{"url", "content_type"} {
		if _, ok := form.Config[name]; !ok {
			ctx.Error(422, "", "Missing config option: "+name)
			return false
		}
	}
	if !models.IsValidHookContentType(form.Config["content_type"]) {
		ctx.Error(422, "", "Invalid content type")
		return false
	}
	return true
}

// AddOrgHook add a hook to an organization. Writes to `ctx` accordingly
func AddOrgHook(ctx *context.APIContext, form *api.CreateHookOption) {
	org := ctx.Org.Organization
	hook, ok := addHook(ctx, form, org.ID, 0)
	if ok {
		ctx.JSON(200, convert.ToHook(org.HomeLink(), hook))
	}
}

// AddRepoHook add a hook to a repo. Writes to `ctx` accordingly
func AddRepoHook(ctx *context.APIContext, form *api.CreateHookOption) {
	repo := ctx.Repo
	hook, ok := addHook(ctx, form, 0, repo.Repository.ID)
	if ok {
		ctx.JSON(200, convert.ToHook(repo.RepoLink, hook))
	}
}

// addHook add the hook specified by `form`, `orgID` and `repoID`. If there is
// an error, write to `ctx` accordingly. Return (webhook, ok)
func addHook(ctx *context.APIContext, form *api.CreateHookOption, orgID, repoID int64) (*models.Webhook, bool) {
	if len(form.Events) == 0 {
		form.Events = []string{"push"}
	}
	w := &models.Webhook{
		OrgID:       orgID,
		RepoID:      repoID,
		URL:         form.Config["url"],
		ContentType: models.ToHookContentType(form.Config["content_type"]),
		Secret:      form.Config["secret"],
		HookEvent: &models.HookEvent{
			ChooseEvents: true,
			HookEvents: models.HookEvents{
				Create:      com.IsSliceContainsStr(form.Events, string(models.HookEventCreate)),
				Push:        com.IsSliceContainsStr(form.Events, string(models.HookEventPush)),
				PullRequest: com.IsSliceContainsStr(form.Events, string(models.HookEventPullRequest)),
			},
		},
		IsActive:     form.Active,
		HookTaskType: models.ToHookTaskType(form.Type),
	}
	if w.HookTaskType == models.SLACK {
		channel, ok := form.Config["channel"]
		if !ok {
			ctx.Error(422, "", "Missing config option: channel")
			return nil, false
		}
		meta, err := json.Marshal(&models.SlackMeta{
			Channel:  channel,
			Username: form.Config["username"],
			IconURL:  form.Config["icon_url"],
			Color:    form.Config["color"],
		})
		if err != nil {
			ctx.Error(500, "slack: JSON marshal failed", err)
			return nil, false
		}
		w.Meta = string(meta)
	}

	if err := w.UpdateEvent(); err != nil {
		ctx.Error(500, "UpdateEvent", err)
		return nil, false
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.Error(500, "CreateWebhook", err)
		return nil, false
	}
	return w, true
}

// EditOrgHook edit webhook `w` according to `form`. Writes to `ctx` accordingly
func EditOrgHook(ctx *context.APIContext, form *api.EditHookOption, hookID int64) {
	org := ctx.Org.Organization
	hook, err := GetOrgHook(ctx, org.ID, hookID)
	if err != nil {
		return
	}
	if !editHook(ctx, form, hook) {
		return
	}
	updated, err := GetOrgHook(ctx, org.ID, hookID)
	if err != nil {
		return
	}
	ctx.JSON(200, convert.ToHook(org.HomeLink(), updated))
}

// EditRepoHook edit webhook `w` according to `form`. Writes to `ctx` accordingly
func EditRepoHook(ctx *context.APIContext, form *api.EditHookOption, hookID int64) {
	repo := ctx.Repo
	hook, err := GetRepoHook(ctx, repo.Repository.ID, hookID)
	if err != nil {
		return
	}
	if !editHook(ctx, form, hook) {
		return
	}
	updated, err := GetRepoHook(ctx, repo.Repository.ID, hookID)
	if err != nil {
		return
	}
	ctx.JSON(200, convert.ToHook(repo.RepoLink, updated))
}

// editHook edit the webhook `w` according to `form`. If an error occurs, write
// to `ctx` accordingly and return the error. Return whether successful
func editHook(ctx *context.APIContext, form *api.EditHookOption, w *models.Webhook) bool {
	if form.Config != nil {
		if url, ok := form.Config["url"]; ok {
			w.URL = url
		}
		if ct, ok := form.Config["content_type"]; ok {
			if !models.IsValidHookContentType(ct) {
				ctx.Error(422, "", "Invalid content type")
				return false
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
					return false
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
	w.Create = com.IsSliceContainsStr(form.Events, string(models.HookEventCreate))
	w.Push = com.IsSliceContainsStr(form.Events, string(models.HookEventPush))
	w.PullRequest = com.IsSliceContainsStr(form.Events, string(models.HookEventPullRequest))
	if err := w.UpdateEvent(); err != nil {
		ctx.Error(500, "UpdateEvent", err)
		return false
	}

	if form.Active != nil {
		w.IsActive = *form.Active
	}

	if err := models.UpdateWebhook(w); err != nil {
		ctx.Error(500, "UpdateWebhook", err)
		return false
	}
	return true
}
