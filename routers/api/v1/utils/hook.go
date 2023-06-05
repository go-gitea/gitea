// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"fmt"
	"net/http"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	webhook_service "code.gitea.io/gitea/services/webhook"
)

// ListOwnerHooks lists the webhooks of the provided owner
func ListOwnerHooks(ctx *context.APIContext, owner *user_model.User) {
	opts := &webhook.ListWebhookOptions{
		ListOptions: GetListOptions(ctx),
		OwnerID:     owner.ID,
	}

	count, err := webhook.CountWebhooksByOpts(opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	hooks, err := webhook.ListWebhooksByOpts(ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	apiHooks := make([]*api.Hook, len(hooks))
	for i, hook := range hooks {
		apiHooks[i], err = webhook_service.ToHook(owner.HomeLink(), hook)
		if err != nil {
			ctx.InternalServerError(err)
			return
		}
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiHooks)
}

// GetOwnerHook gets an user or organization webhook. Errors are written to ctx.
func GetOwnerHook(ctx *context.APIContext, ownerID, hookID int64) (*webhook.Webhook, error) {
	w, err := webhook.GetWebhookByOwnerID(ownerID, hookID)
	if err != nil {
		if webhook.IsErrWebhookNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetWebhookByOwnerID", err)
		}
		return nil, err
	}
	return w, nil
}

// GetRepoHook get a repo's webhook. If there is an error, write to `ctx`
// accordingly and return the error
func GetRepoHook(ctx *context.APIContext, repoID, hookID int64) (*webhook.Webhook, error) {
	w, err := webhook.GetWebhookByRepoID(repoID, hookID)
	if err != nil {
		if webhook.IsErrWebhookNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetWebhookByID", err)
		}
		return nil, err
	}
	return w, nil
}

// checkCreateHookOption check if a CreateHookOption form is valid. If invalid,
// write the appropriate error to `ctx`. Return whether the form is valid
func checkCreateHookOption(ctx *context.APIContext, form *api.CreateHookOption) bool {
	if !webhook_service.IsValidHookTaskType(form.Type) {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("Invalid hook type: %s", form.Type))
		return false
	}
	for _, name := range []string{"url", "content_type"} {
		if _, ok := form.Config[name]; !ok {
			ctx.Error(http.StatusUnprocessableEntity, "", "Missing config option: "+name)
			return false
		}
	}
	if !webhook.IsValidHookContentType(form.Config["content_type"]) {
		ctx.Error(http.StatusUnprocessableEntity, "", "Invalid content type")
		return false
	}
	return true
}

// AddSystemHook add a system hook
func AddSystemHook(ctx *context.APIContext, form *api.CreateHookOption) {
	hook, ok := addHook(ctx, form, 0, 0)
	if ok {
		h, err := webhook_service.ToHook(setting.AppSubURL+"/admin", hook)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "convert.ToHook", err)
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
		ctx.Error(http.StatusInternalServerError, "ToHook", err)
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

// addHook add the hook specified by `form`, `ownerID` and `repoID`. If there is
// an error, write to `ctx` accordingly. Return (webhook, ok)
func addHook(ctx *context.APIContext, form *api.CreateHookOption, ownerID, repoID int64) (*webhook.Webhook, bool) {
	if !checkCreateHookOption(ctx, form) {
		return nil, false
	}

	if len(form.Events) == 0 {
		form.Events = []string{"push"}
	}
	w := &webhook.Webhook{
		OwnerID:     ownerID,
		RepoID:      repoID,
		URL:         form.Config["url"],
		ContentType: webhook.ToHookContentType(form.Config["content_type"]),
		Secret:      form.Config["secret"],
		HTTPMethod:  "POST",
		HookEvent: &webhook_module.HookEvent{
			ChooseEvents: true,
			HookEvents: webhook_module.HookEvents{
				Create:                   util.SliceContainsString(form.Events, string(webhook_module.HookEventCreate), true),
				Delete:                   util.SliceContainsString(form.Events, string(webhook_module.HookEventDelete), true),
				Fork:                     util.SliceContainsString(form.Events, string(webhook_module.HookEventFork), true),
				Issues:                   issuesHook(form.Events, "issues_only"),
				IssueAssign:              issuesHook(form.Events, string(webhook_module.HookEventIssueAssign)),
				IssueLabel:               issuesHook(form.Events, string(webhook_module.HookEventIssueLabel)),
				IssueMilestone:           issuesHook(form.Events, string(webhook_module.HookEventIssueMilestone)),
				IssueComment:             issuesHook(form.Events, string(webhook_module.HookEventIssueComment)),
				Push:                     util.SliceContainsString(form.Events, string(webhook_module.HookEventPush), true),
				PullRequest:              pullHook(form.Events, "pull_request_only"),
				PullRequestAssign:        pullHook(form.Events, string(webhook_module.HookEventPullRequestAssign)),
				PullRequestLabel:         pullHook(form.Events, string(webhook_module.HookEventPullRequestLabel)),
				PullRequestMilestone:     pullHook(form.Events, string(webhook_module.HookEventPullRequestMilestone)),
				PullRequestComment:       pullHook(form.Events, string(webhook_module.HookEventPullRequestComment)),
				PullRequestReview:        pullHook(form.Events, "pull_request_review"),
				PullRequestReviewRequest: pullHook(form.Events, string(webhook_module.HookEventPullRequestReviewRequest)),
				PullRequestSync:          pullHook(form.Events, string(webhook_module.HookEventPullRequestSync)),
				Wiki:                     util.SliceContainsString(form.Events, string(webhook_module.HookEventWiki), true),
				Repository:               util.SliceContainsString(form.Events, string(webhook_module.HookEventRepository), true),
				Release:                  util.SliceContainsString(form.Events, string(webhook_module.HookEventRelease), true),
			},
			BranchFilter: form.BranchFilter,
		},
		IsActive: form.Active,
		Type:     form.Type,
	}
	err := w.SetHeaderAuthorization(form.AuthorizationHeader)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SetHeaderAuthorization", err)
		return nil, false
	}
	if w.Type == webhook_module.SLACK {
		channel, ok := form.Config["channel"]
		if !ok {
			ctx.Error(http.StatusUnprocessableEntity, "", "Missing config option: channel")
			return nil, false
		}
		channel = strings.TrimSpace(channel)

		if !webhook_service.IsValidSlackChannel(channel) {
			ctx.Error(http.StatusBadRequest, "", "Invalid slack channel name")
			return nil, false
		}

		meta, err := json.Marshal(&webhook_service.SlackMeta{
			Channel:  channel,
			Username: form.Config["username"],
			IconURL:  form.Config["icon_url"],
			Color:    form.Config["color"],
		})
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "slack: JSON marshal failed", err)
			return nil, false
		}
		w.Meta = string(meta)
	}

	if err := w.UpdateEvent(); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateEvent", err)
		return nil, false
	} else if err := webhook.CreateWebhook(ctx, w); err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateWebhook", err)
		return nil, false
	}
	return w, true
}

// EditSystemHook edit system webhook `w` according to `form`. Writes to `ctx` accordingly
func EditSystemHook(ctx *context.APIContext, form *api.EditHookOption, hookID int64) {
	hook, err := webhook.GetSystemOrDefaultWebhook(ctx, hookID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetSystemOrDefaultWebhook", err)
		return
	}
	if !editHook(ctx, form, hook) {
		ctx.Error(http.StatusInternalServerError, "editHook", err)
		return
	}
	updated, err := webhook.GetSystemOrDefaultWebhook(ctx, hookID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetSystemOrDefaultWebhook", err)
		return
	}
	h, err := webhook_service.ToHook(setting.AppURL+"/admin", updated)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convert.ToHook", err)
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
			w.URL = url
		}
		if ct, ok := form.Config["content_type"]; ok {
			if !webhook.IsValidHookContentType(ct) {
				ctx.Error(http.StatusUnprocessableEntity, "", "Invalid content type")
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
					ctx.Error(http.StatusInternalServerError, "slack: JSON marshal failed", err)
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
	w.Create = util.SliceContainsString(form.Events, string(webhook_module.HookEventCreate), true)
	w.Push = util.SliceContainsString(form.Events, string(webhook_module.HookEventPush), true)
	w.Create = util.SliceContainsString(form.Events, string(webhook_module.HookEventCreate), true)
	w.Delete = util.SliceContainsString(form.Events, string(webhook_module.HookEventDelete), true)
	w.Fork = util.SliceContainsString(form.Events, string(webhook_module.HookEventFork), true)
	w.Repository = util.SliceContainsString(form.Events, string(webhook_module.HookEventRepository), true)
	w.Wiki = util.SliceContainsString(form.Events, string(webhook_module.HookEventWiki), true)
	w.Release = util.SliceContainsString(form.Events, string(webhook_module.HookEventRelease), true)
	w.BranchFilter = form.BranchFilter

	err := w.SetHeaderAuthorization(form.AuthorizationHeader)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SetHeaderAuthorization", err)
		return false
	}

	// Issues
	w.Issues = issuesHook(form.Events, "issues_only")
	w.IssueAssign = issuesHook(form.Events, string(webhook_module.HookEventIssueAssign))
	w.IssueLabel = issuesHook(form.Events, string(webhook_module.HookEventIssueLabel))
	w.IssueMilestone = issuesHook(form.Events, string(webhook_module.HookEventIssueMilestone))
	w.IssueComment = issuesHook(form.Events, string(webhook_module.HookEventIssueComment))

	// Pull requests
	w.PullRequest = pullHook(form.Events, "pull_request_only")
	w.PullRequestAssign = pullHook(form.Events, string(webhook_module.HookEventPullRequestAssign))
	w.PullRequestLabel = pullHook(form.Events, string(webhook_module.HookEventPullRequestLabel))
	w.PullRequestMilestone = pullHook(form.Events, string(webhook_module.HookEventPullRequestMilestone))
	w.PullRequestComment = pullHook(form.Events, string(webhook_module.HookEventPullRequestComment))
	w.PullRequestReview = pullHook(form.Events, "pull_request_review")
	w.PullRequestReviewRequest = pullHook(form.Events, string(webhook_module.HookEventPullRequestReviewRequest))
	w.PullRequestSync = pullHook(form.Events, string(webhook_module.HookEventPullRequestSync))

	if err := w.UpdateEvent(); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateEvent", err)
		return false
	}

	if form.Active != nil {
		w.IsActive = *form.Active
	}

	if err := webhook.UpdateWebhook(w); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateWebhook", err)
		return false
	}
	return true
}

// DeleteOwnerHook deletes the hook owned by the owner.
func DeleteOwnerHook(ctx *context.APIContext, owner *user_model.User, hookID int64) {
	if err := webhook.DeleteWebhookByOwnerID(owner.ID, hookID); err != nil {
		if webhook.IsErrWebhookNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteWebhookByOwnerID", err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}
