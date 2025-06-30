// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/services/context"
	webhook_service "code.gitea.io/gitea/services/webhook"
)

// ListOwnerHooks lists the webhooks of the provided owner
func ListOwnerHooks(ctx *context.APIContext, owner *user_model.User) {
	opts := &webhook.ListWebhookOptions{
		ListOptions: GetListOptions(ctx),
		OwnerID:     owner.ID,
	}

	hooks, count, err := db.FindAndCount[webhook.Webhook](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiHooks := make([]*api.Hook, len(hooks))
	for i, hook := range hooks {
		apiHooks[i], err = webhook_service.ToHook(owner.HomeLink(), hook)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiHooks)
}

// GetOwnerHook gets an user or organization webhook. Errors are written to ctx.
func GetOwnerHook(ctx *context.APIContext, ownerID, hookID int64) (*webhook.Webhook, error) {
	w, err := webhook.GetWebhookByOwnerID(ctx, ownerID, hookID)
	if err != nil {
		if webhook.IsErrWebhookNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil, err
	}
	return w, nil
}

// GetRepoHook get a repo's webhook. If there is an error, write to `ctx`
// accordingly and return the error
func GetRepoHook(ctx *context.APIContext, repoID, hookID int64) (*webhook.Webhook, error) {
	w, err := webhook.GetWebhookByRepoID(ctx, repoID, hookID)
	if err != nil {
		if webhook.IsErrWebhookNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil, err
	}
	return w, nil
}

// checkCreateHookOption check if a CreateHookOption form is valid. If invalid,
// write the appropriate error to `ctx`. Return whether the form is valid
func checkCreateHookOption(ctx *context.APIContext, form *api.CreateHookOption) bool {
	if !webhook_service.IsValidHookTaskType(form.Type) {
		ctx.APIError(http.StatusUnprocessableEntity, "Invalid hook type: "+form.Type)
		return false
	}
	for _, name := range []string{"url", "content_type"} {
		if _, ok := form.Config[name]; !ok {
			ctx.APIError(http.StatusUnprocessableEntity, "Missing config option: "+name)
			return false
		}
	}
	if !webhook.IsValidHookContentType(form.Config["content_type"]) {
		ctx.APIError(http.StatusUnprocessableEntity, "Invalid content type")
		return false
	}
	if !validation.IsValidURL(form.Config["url"]) {
		ctx.APIError(http.StatusUnprocessableEntity, "Invalid url")
		return false
	}
	return true
}

// AddSystemHook add a system hook
func AddSystemHook(ctx *context.APIContext, form *api.CreateHookOption) {
	hook, ok := addHook(ctx, form, 0, 0)
	if ok {
		h, err := webhook_service.ToHook(setting.AppSubURL+"/-/admin", hook)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		ctx.JSON(http.StatusCreated, h)
	}
}

// AddOwnerHook adds a hook to an user or organization
func AddOwnerHook(ctx *context.APIContext, owner *user_model.User, form *api.CreateHookOption) {
	hook, ok := addHook(ctx, form, owner.ID, 0)
	if !ok {
		return
	}
	apiHook, ok := toAPIHook(ctx, owner.HomeLink(), hook)
	if !ok {
		return
	}
	ctx.JSON(http.StatusCreated, apiHook)
}

// AddRepoHook add a hook to a repo. Writes to `ctx` accordingly
func AddRepoHook(ctx *context.APIContext, form *api.CreateHookOption) {
	repo := ctx.Repo
	hook, ok := addHook(ctx, form, 0, repo.Repository.ID)
	if !ok {
		return
	}
	apiHook, ok := toAPIHook(ctx, repo.RepoLink, hook)
	if !ok {
		return
	}
	ctx.JSON(http.StatusCreated, apiHook)
}

// toAPIHook converts the hook to its API representation.
// If there is an error, write to `ctx` accordingly. Return (hook, ok)
func toAPIHook(ctx *context.APIContext, repoLink string, hook *webhook.Webhook) (*api.Hook, bool) {
	apiHook, err := webhook_service.ToHook(repoLink, hook)
	if err != nil {
		ctx.APIErrorInternal(err)
		return nil, false
	}
	return apiHook, true
}

func issuesHook(events []string, event string) bool {
	return util.SliceContainsString(events, event, true) || util.SliceContainsString(events, string(webhook_module.HookEventIssues), true)
}

func pullHook(events []string, event string) bool {
	return util.SliceContainsString(events, event, true) || util.SliceContainsString(events, string(webhook_module.HookEventPullRequest), true)
}

func updateHookEvents(events []string) webhook_module.HookEvents {
	if len(events) == 0 {
		events = []string{"push"}
	}
	hookEvents := make(webhook_module.HookEvents)
	hookEvents[webhook_module.HookEventCreate] = util.SliceContainsString(events, string(webhook_module.HookEventCreate), true)
	hookEvents[webhook_module.HookEventPush] = util.SliceContainsString(events, string(webhook_module.HookEventPush), true)
	hookEvents[webhook_module.HookEventDelete] = util.SliceContainsString(events, string(webhook_module.HookEventDelete), true)
	hookEvents[webhook_module.HookEventFork] = util.SliceContainsString(events, string(webhook_module.HookEventFork), true)
	hookEvents[webhook_module.HookEventRepository] = util.SliceContainsString(events, string(webhook_module.HookEventRepository), true)
	hookEvents[webhook_module.HookEventWiki] = util.SliceContainsString(events, string(webhook_module.HookEventWiki), true)
	hookEvents[webhook_module.HookEventRelease] = util.SliceContainsString(events, string(webhook_module.HookEventRelease), true)
	hookEvents[webhook_module.HookEventPackage] = util.SliceContainsString(events, string(webhook_module.HookEventPackage), true)
	hookEvents[webhook_module.HookEventStatus] = util.SliceContainsString(events, string(webhook_module.HookEventStatus), true)
	hookEvents[webhook_module.HookEventWorkflowRun] = util.SliceContainsString(events, string(webhook_module.HookEventWorkflowRun), true)
	hookEvents[webhook_module.HookEventWorkflowJob] = util.SliceContainsString(events, string(webhook_module.HookEventWorkflowJob), true)

	// Issues
	hookEvents[webhook_module.HookEventIssues] = issuesHook(events, "issues_only")
	hookEvents[webhook_module.HookEventIssueAssign] = issuesHook(events, string(webhook_module.HookEventIssueAssign))
	hookEvents[webhook_module.HookEventIssueLabel] = issuesHook(events, string(webhook_module.HookEventIssueLabel))
	hookEvents[webhook_module.HookEventIssueMilestone] = issuesHook(events, string(webhook_module.HookEventIssueMilestone))
	hookEvents[webhook_module.HookEventIssueComment] = issuesHook(events, string(webhook_module.HookEventIssueComment))

	// Pull requests
	hookEvents[webhook_module.HookEventPullRequest] = pullHook(events, "pull_request_only")
	hookEvents[webhook_module.HookEventPullRequestAssign] = pullHook(events, string(webhook_module.HookEventPullRequestAssign))
	hookEvents[webhook_module.HookEventPullRequestLabel] = pullHook(events, string(webhook_module.HookEventPullRequestLabel))
	hookEvents[webhook_module.HookEventPullRequestMilestone] = pullHook(events, string(webhook_module.HookEventPullRequestMilestone))
	hookEvents[webhook_module.HookEventPullRequestComment] = pullHook(events, string(webhook_module.HookEventPullRequestComment))
	hookEvents[webhook_module.HookEventPullRequestReview] = pullHook(events, "pull_request_review")
	hookEvents[webhook_module.HookEventPullRequestReviewRequest] = pullHook(events, string(webhook_module.HookEventPullRequestReviewRequest))
	hookEvents[webhook_module.HookEventPullRequestSync] = pullHook(events, string(webhook_module.HookEventPullRequestSync))
	return hookEvents
}

// addHook add the hook specified by `form`, `ownerID` and `repoID`. If there is
// an error, write to `ctx` accordingly. Return (webhook, ok)
func addHook(ctx *context.APIContext, form *api.CreateHookOption, ownerID, repoID int64) (*webhook.Webhook, bool) {
	var isSystemWebhook bool
	if !checkCreateHookOption(ctx, form) {
		return nil, false
	}

	if form.Config["is_system_webhook"] != "" {
		sw, err := strconv.ParseBool(form.Config["is_system_webhook"])
		if err != nil {
			ctx.APIError(http.StatusUnprocessableEntity, "Invalid is_system_webhook value")
			return nil, false
		}
		isSystemWebhook = sw
	}
	w := &webhook.Webhook{
		OwnerID:         ownerID,
		RepoID:          repoID,
		URL:             form.Config["url"],
		ContentType:     webhook.ToHookContentType(form.Config["content_type"]),
		Secret:          form.Config["secret"],
		HTTPMethod:      "POST",
		IsSystemWebhook: isSystemWebhook,
		HookEvent: &webhook_module.HookEvent{
			ChooseEvents: true,
			HookEvents:   updateHookEvents(form.Events),
			BranchFilter: form.BranchFilter,
		},
		IsActive: form.Active,
		Type:     form.Type,
	}
	err := w.SetHeaderAuthorization(form.AuthorizationHeader)
	if err != nil {
		ctx.APIErrorInternal(err)
		return nil, false
	}
	if w.Type == webhook_module.SLACK {
		channel, ok := form.Config["channel"]
		if !ok {
			ctx.APIError(http.StatusUnprocessableEntity, "Missing config option: channel")
			return nil, false
		}
		channel = strings.TrimSpace(channel)

		if !webhook_service.IsValidSlackChannel(channel) {
			ctx.APIError(http.StatusBadRequest, "Invalid slack channel name")
			return nil, false
		}

		meta, err := json.Marshal(&webhook_service.SlackMeta{
			Channel:  channel,
			Username: form.Config["username"],
			IconURL:  form.Config["icon_url"],
			Color:    form.Config["color"],
		})
		if err != nil {
			ctx.APIErrorInternal(err)
			return nil, false
		}
		w.Meta = string(meta)
	}

	if err := w.UpdateEvent(); err != nil {
		ctx.APIErrorInternal(err)
		return nil, false
	} else if err := webhook.CreateWebhook(ctx, w); err != nil {
		ctx.APIErrorInternal(err)
		return nil, false
	}
	return w, true
}

// EditSystemHook edit system webhook `w` according to `form`. Writes to `ctx` accordingly
func EditSystemHook(ctx *context.APIContext, form *api.EditHookOption, hookID int64) {
	hook, err := webhook.GetSystemOrDefaultWebhook(ctx, hookID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if !editHook(ctx, form, hook) {
		ctx.APIErrorInternal(err)
		return
	}
	updated, err := webhook.GetSystemOrDefaultWebhook(ctx, hookID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	h, err := webhook_service.ToHook(setting.AppURL+"/-/admin", updated)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, h)
}

// EditOwnerHook updates a webhook of an user or organization
func EditOwnerHook(ctx *context.APIContext, owner *user_model.User, form *api.EditHookOption, hookID int64) {
	hook, err := GetOwnerHook(ctx, owner.ID, hookID)
	if err != nil {
		return
	}
	if !editHook(ctx, form, hook) {
		return
	}
	updated, err := GetOwnerHook(ctx, owner.ID, hookID)
	if err != nil {
		return
	}
	apiHook, ok := toAPIHook(ctx, owner.HomeLink(), updated)
	if !ok {
		return
	}
	ctx.JSON(http.StatusOK, apiHook)
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
	apiHook, ok := toAPIHook(ctx, repo.RepoLink, updated)
	if !ok {
		return
	}
	ctx.JSON(http.StatusOK, apiHook)
}

// editHook edit the webhook `w` according to `form`. If an error occurs, write
// to `ctx` accordingly and return the error. Return whether successful
func editHook(ctx *context.APIContext, form *api.EditHookOption, w *webhook.Webhook) bool {
	if form.Config != nil {
		if url, ok := form.Config["url"]; ok {
			if !validation.IsValidURL(url) {
				ctx.APIError(http.StatusUnprocessableEntity, "Invalid url")
				return false
			}
			w.URL = url
		}
		if ct, ok := form.Config["content_type"]; ok {
			if !webhook.IsValidHookContentType(ct) {
				ctx.APIError(http.StatusUnprocessableEntity, "Invalid content type")
				return false
			}
			w.ContentType = webhook.ToHookContentType(ct)
		}

		if w.Type == webhook_module.SLACK {
			if channel, ok := form.Config["channel"]; ok {
				meta, err := json.Marshal(&webhook_service.SlackMeta{
					Channel:  channel,
					Username: form.Config["username"],
					IconURL:  form.Config["icon_url"],
					Color:    form.Config["color"],
				})
				if err != nil {
					ctx.APIErrorInternal(err)
					return false
				}
				w.Meta = string(meta)
			}
		}
	}

	// Update events
	w.HookEvents = updateHookEvents(form.Events)
	w.PushOnly = false
	w.SendEverything = false
	w.ChooseEvents = true
	w.BranchFilter = form.BranchFilter

	err := w.SetHeaderAuthorization(form.AuthorizationHeader)
	if err != nil {
		ctx.APIErrorInternal(err)
		return false
	}

	if err := w.UpdateEvent(); err != nil {
		ctx.APIErrorInternal(err)
		return false
	}

	if form.Active != nil {
		w.IsActive = *form.Active
	}

	if err := webhook.UpdateWebhook(ctx, w); err != nil {
		ctx.APIErrorInternal(err)
		return false
	}
	return true
}

// DeleteOwnerHook deletes the hook owned by the owner.
func DeleteOwnerHook(ctx *context.APIContext, owner *user_model.User, hookID int64) {
	if err := webhook.DeleteWebhookByOwnerID(ctx, owner.ID, hookID); err != nil {
		if webhook.IsErrWebhookNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}
