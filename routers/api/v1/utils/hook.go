// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	webhook_service "code.gitea.io/gitea/services/webhook"
)

// GetOrgHook get an organization's webhook. If there is an error, write to
// `ctx` accordingly and return the error
func GetOrgHook(ctx *context.APIContext, orgID, hookID int64) (*webhook.Webhook, error) {
	w, err := webhook.GetWebhookByOrgID(orgID, hookID)
	if err != nil {
		if webhook.IsErrWebhookNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetWebhookByOrgID", err)
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

// CheckCreateHookOption check if a CreateHookOption form is valid. If invalid,
// write the appropriate error to `ctx`. Return whether the form is valid
func CheckCreateHookOption(ctx *context.APIContext, form *api.CreateHookOption) bool {
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

// AddOrgHook add a hook to an organization. Writes to `ctx` accordingly
func AddOrgHook(ctx *context.APIContext, form *api.CreateHookOption) {
	org := ctx.Org.Organization
	hook, ok := addHook(ctx, form, org.ID, 0)
	if ok {
		ctx.JSON(http.StatusCreated, convert.ToHook(org.AsUser().HomeLink(), hook))
	}
}

// AddRepoHook add a hook to a repo. Writes to `ctx` accordingly
func AddRepoHook(ctx *context.APIContext, form *api.CreateHookOption) {
	repo := ctx.Repo
	hook, ok := addHook(ctx, form, 0, repo.Repository.ID)
	if ok {
		ctx.JSON(http.StatusCreated, convert.ToHook(repo.RepoLink, hook))
	}
}

func issuesHook(events []string, event string) bool {
	return util.IsStringInSlice(event, events, true) || util.IsStringInSlice(string(webhook.HookEventIssues), events, true)
}

func pullHook(events []string, event string) bool {
	return util.IsStringInSlice(event, events, true) || util.IsStringInSlice(string(webhook.HookEventPullRequest), events, true)
}

// addHook add the hook specified by `form`, `orgID` and `repoID`. If there is
// an error, write to `ctx` accordingly. Return (webhook, ok)
func addHook(ctx *context.APIContext, form *api.CreateHookOption, orgID, repoID int64) (*webhook.Webhook, bool) {
	if len(form.Events) == 0 {
		form.Events = []string{"push"}
	}
	w := &webhook.Webhook{
		OrgID:       orgID,
		RepoID:      repoID,
		URL:         form.Config["url"],
		ContentType: webhook.ToHookContentType(form.Config["content_type"]),
		Secret:      form.Config["secret"],
		HTTPMethod:  "POST",
		HookEvent: &webhook.HookEvent{
			ChooseEvents: true,
			HookEvents: webhook.HookEvents{
				Create:               util.IsStringInSlice(string(webhook.HookEventCreate), form.Events, true),
				Delete:               util.IsStringInSlice(string(webhook.HookEventDelete), form.Events, true),
				Fork:                 util.IsStringInSlice(string(webhook.HookEventFork), form.Events, true),
				Issues:               issuesHook(form.Events, "issues_only"),
				IssueAssign:          issuesHook(form.Events, string(webhook.HookEventIssueAssign)),
				IssueLabel:           issuesHook(form.Events, string(webhook.HookEventIssueLabel)),
				IssueMilestone:       issuesHook(form.Events, string(webhook.HookEventIssueMilestone)),
				IssueComment:         issuesHook(form.Events, string(webhook.HookEventIssueComment)),
				Push:                 util.IsStringInSlice(string(webhook.HookEventPush), form.Events, true),
				PullRequest:          pullHook(form.Events, "pull_request_only"),
				PullRequestAssign:    pullHook(form.Events, string(webhook.HookEventPullRequestAssign)),
				PullRequestLabel:     pullHook(form.Events, string(webhook.HookEventPullRequestLabel)),
				PullRequestMilestone: pullHook(form.Events, string(webhook.HookEventPullRequestMilestone)),
				PullRequestComment:   pullHook(form.Events, string(webhook.HookEventPullRequestComment)),
				PullRequestReview:    pullHook(form.Events, "pull_request_review"),
				PullRequestSync:      pullHook(form.Events, string(webhook.HookEventPullRequestSync)),
				Wiki:                 util.IsStringInSlice(string(webhook.HookEventWiki), form.Events, true),
				Repository:           util.IsStringInSlice(string(webhook.HookEventRepository), form.Events, true),
				Release:              util.IsStringInSlice(string(webhook.HookEventRelease), form.Events, true),
			},
			BranchFilter: form.BranchFilter,
		},
		IsActive: form.Active,
		Type:     form.Type,
	}
	if w.Type == webhook.SLACK {
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
	ctx.JSON(http.StatusOK, convert.ToHook(org.AsUser().HomeLink(), updated))
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
	ctx.JSON(http.StatusOK, convert.ToHook(repo.RepoLink, updated))
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

		if w.Type == webhook.SLACK {
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
	w.Create = util.IsStringInSlice(string(webhook.HookEventCreate), form.Events, true)
	w.Push = util.IsStringInSlice(string(webhook.HookEventPush), form.Events, true)
	w.Create = util.IsStringInSlice(string(webhook.HookEventCreate), form.Events, true)
	w.Delete = util.IsStringInSlice(string(webhook.HookEventDelete), form.Events, true)
	w.Fork = util.IsStringInSlice(string(webhook.HookEventFork), form.Events, true)
	w.Repository = util.IsStringInSlice(string(webhook.HookEventRepository), form.Events, true)
	w.Wiki = util.IsStringInSlice(string(webhook.HookEventWiki), form.Events, true)
	w.Release = util.IsStringInSlice(string(webhook.HookEventRelease), form.Events, true)
	w.BranchFilter = form.BranchFilter

	// Issues
	w.Issues = issuesHook(form.Events, "issues_only")
	w.IssueAssign = issuesHook(form.Events, string(webhook.HookEventIssueAssign))
	w.IssueLabel = issuesHook(form.Events, string(webhook.HookEventIssueLabel))
	w.IssueMilestone = issuesHook(form.Events, string(webhook.HookEventIssueMilestone))
	w.IssueComment = issuesHook(form.Events, string(webhook.HookEventIssueComment))

	// Pull requests
	w.PullRequest = pullHook(form.Events, "pull_request_only")
	w.PullRequestAssign = pullHook(form.Events, string(webhook.HookEventPullRequestAssign))
	w.PullRequestLabel = pullHook(form.Events, string(webhook.HookEventPullRequestLabel))
	w.PullRequestMilestone = pullHook(form.Events, string(webhook.HookEventPullRequestMilestone))
	w.PullRequestComment = pullHook(form.Events, string(webhook.HookEventPullRequestComment))
	w.PullRequestReview = pullHook(form.Events, "pull_request_review")
	w.PullRequestSync = pullHook(form.Events, string(webhook.HookEventPullRequestSync))

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
