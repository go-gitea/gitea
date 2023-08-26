// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

// ListOwnerHooks lists the webhooks of the provided owner
func ListOwnerHooks(ctx context.Context, listOptions db.ListOptions, owner *user_model.User) ([]*api.Hook, int64, error) {
	opts := &webhook_model.ListWebhookOptions{
		ListOptions: listOptions,
		OwnerID:     owner.ID,
	}

	count, err := webhook_model.CountWebhooksByOpts(opts)
	if err != nil {
		return nil, 0, err
	}

	hooks, err := webhook_model.ListWebhooksByOpts(ctx, opts)
	if err != nil {
		return nil, 0, err
	}

	apiHooks := make([]*api.Hook, len(hooks))
	for i, hook := range hooks {
		apiHooks[i], err = ToHook(owner.HomeLink(), hook)
		if err != nil {
			return nil, 0, err
		}
	}

	return apiHooks, count, nil
}

// GetOwnerHook gets an user or organization webhook.
func GetOwnerHook(ownerID, hookID int64) (*webhook_model.Webhook, error) {
	webhook, err := webhook_model.GetWebhookByOwnerID(ownerID, hookID)
	if err != nil {
		return nil, err
	}
	return webhook, nil
}

// GetRepoHook get a repo's webhook.
func GetRepoHook(repoID, hookID int64) (*webhook_model.Webhook, error) {
	webhook, err := webhook_model.GetWebhookByRepoID(repoID, hookID)
	if err != nil {
		return nil, err
	}
	return webhook, nil
}

// checkCreateHookOption check if a CreateHookOption form is valid.
func checkCreateHookOption(form *api.CreateHookOption) error {
	if !IsValidHookTaskType(form.Type) {
		return fmt.Errorf("invalid hook type: %s", form.Type)
	}
	for _, name := range []string{"url", "content_type"} {
		if _, ok := form.Config[name]; !ok {
			return fmt.Errorf("missing config option: %s", name)
		}
	}
	if !webhook_model.IsValidHookContentType(form.Config["content_type"]) {
		return fmt.Errorf("invalid content type: %s", form.Config["content_type"])
	}
	return nil
}

// AddSystemHook add a system hook
func AddSystemHook(ctx context.Context, form *api.CreateHookOption) (apiHook *api.Hook, httpStatus int, logTitle string, err error) {
	hook, httpStatus, logTitle, err := addHook(ctx, form, 0, 0)
	if err != nil {
		return nil, httpStatus, logTitle, err
	}

	apiHook, err = ToHook(setting.AppSubURL+"/admin", hook)
	if err != nil {
		return nil, http.StatusInternalServerError, "ToHook", err
	}
	return apiHook, http.StatusCreated, "", nil
}

// AddOwnerHook adds a hook to an user or organization
func AddOwnerHook(ctx context.Context, owner *user_model.User, form *api.CreateHookOption) (apiHook *api.Hook, httpStatus int, logTitle string, err error) {
	hook, httpStatus, logTitle, err := addHook(ctx, form, owner.ID, 0)
	if err != nil {
		return nil, httpStatus, logTitle, err
	}

	apiHook, err = ToHook(owner.HomeLink(), hook)
	if err != nil {
		return nil, http.StatusInternalServerError, "ToHook", err
	}
	return apiHook, http.StatusCreated, "", nil
}

// AddRepoHook add a hook to a repo.
func AddRepoHook(ctx context.Context, repoID int64, repoLink string, form *api.CreateHookOption) (apiHook *api.Hook, httpStatus int, logTitle string, err error) {
	hook, httpStatus, logTitle, err := addHook(ctx, form, 0, repoID)
	if err != nil {
		return nil, httpStatus, logTitle, err
	}

	apiHook, err = ToHook(repoLink, hook)
	if err != nil {
		return nil, http.StatusInternalServerError, "ToHook", err
	}
	return apiHook, http.StatusCreated, "", nil
}

func issuesHook(events []string, event string) bool {
	return util.SliceContainsString(events, event, true) || util.SliceContainsString(events, string(webhook_module.HookEventIssues), true)
}

func pullHook(events []string, event string) bool {
	return util.SliceContainsString(events, event, true) || util.SliceContainsString(events, string(webhook_module.HookEventPullRequest), true)
}

// addHook add the hook specified by `form`, `ownerID` and `repoID`.
func addHook(ctx context.Context, form *api.CreateHookOption, ownerID, repoID int64) (hook *webhook_model.Webhook, httpStatus int, logTitle string, err error) {
	if err = checkCreateHookOption(form); err != nil {
		return nil, http.StatusUnprocessableEntity, "", err
	}

	if len(form.Events) == 0 {
		form.Events = []string{"push"}
	}
	w := &webhook_model.Webhook{
		OwnerID:     ownerID,
		RepoID:      repoID,
		URL:         form.Config["url"],
		ContentType: webhook_model.ToHookContentType(form.Config["content_type"]),
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
	err = w.SetHeaderAuthorization(form.AuthorizationHeader)
	if err != nil {
		return nil, http.StatusInternalServerError, "SetHeaderAuthorization", err
	}
	if w.Type == webhook_module.SLACK {
		channel, ok := form.Config["channel"]
		if !ok {
			return nil, http.StatusUnprocessableEntity, "", errors.New("missing config option: channel")
		}
		channel = strings.TrimSpace(channel)

		if !IsValidSlackChannel(channel) {
			return nil, http.StatusBadRequest, "", errors.New("invalid slack channel name")
		}

		meta, err := json.Marshal(&SlackMeta{
			Channel:  channel,
			Username: form.Config["username"],
			IconURL:  form.Config["icon_url"],
			Color:    form.Config["color"],
		})
		if err != nil {
			return nil, http.StatusInternalServerError, "slack: JSON marshal failed", err
		}
		w.Meta = string(meta)
	}

	if err := w.UpdateEvent(); err != nil {
		return nil, http.StatusInternalServerError, "UpdateEvent", err
	} else if err := webhook_model.CreateWebhook(ctx, w); err != nil {
		return nil, http.StatusInternalServerError, "CreateWebhook", err
	}
	return w, http.StatusOK, "", nil
}

// EditSystemHook edit system webhook according to `form`.
func EditSystemHook(ctx context.Context, form *api.EditHookOption, hookID int64) (apiHook *api.Hook, httpStatus int, logTitle string, err error) {
	hook, err := webhook_model.GetSystemOrDefaultWebhook(ctx, hookID)
	if err != nil {
		return nil, http.StatusInternalServerError, "GetSystemOrDefaultWebhook", err
	}
	httpStatus, logTitle, err = editHook(form, hook)
	if err != nil {
		return nil, httpStatus, logTitle, err
	}
	updated, err := webhook_model.GetSystemOrDefaultWebhook(ctx, hookID)
	if err != nil {
		return nil, http.StatusInternalServerError, "GetSystemOrDefaultWebhook", err
	}
	apiHook, err = ToHook(setting.AppURL+"/admin", updated)
	if err != nil {
		return nil, http.StatusInternalServerError, "ToHook", err
	}
	return apiHook, http.StatusOK, "", nil
}

// EditOwnerHook updates a webhook of an user or organization
func EditOwnerHook(ctx context.Context, owner *user_model.User, form *api.EditHookOption, hookID int64) (apiHook *api.Hook, httpStatus int, logTitle string, err error) {
	hook, err := GetOwnerHook(owner.ID, hookID)
	if err != nil {
		return nil, http.StatusInternalServerError, "GetOwnerHook", err
	}
	httpStatus, logTitle, err = editHook(form, hook)
	if err != nil {
		return nil, httpStatus, logTitle, err
	}
	updated, err := GetOwnerHook(owner.ID, hookID)
	if err != nil {
		return
	}
	apiHook, err = ToHook(owner.HomeLink(), updated)
	if err != nil {
		return nil, http.StatusInternalServerError, "ToHook", err
	}
	return apiHook, http.StatusOK, "", nil
}

// EditRepoHook edit webhook `w` according to `form`. Writes to `ctx` accordingly
func EditRepoHook(ctx context.Context, form *api.EditHookOption, repoID int64, repoLink string, hookID int64) (apiHook *api.Hook, httpStatus int, logTitle string, err error) {
	hook, err := GetRepoHook(repoID, hookID)
	if err != nil {
		return
	}
	httpStatus, logTitle, err = editHook(form, hook)
	if err != nil {
		return nil, httpStatus, logTitle, err
	}
	updated, err := GetRepoHook(repoID, hookID)
	if err != nil {
		return
	}
	apiHook, err = ToHook(repoLink, updated)
	if err != nil {
		return nil, http.StatusInternalServerError, "ToHook", err
	}
	return apiHook, http.StatusOK, "", nil
}

// editHook edit the webhook `w` according to `form`.
func editHook(form *api.EditHookOption, w *webhook_model.Webhook) (httpStatus int, logTitle string, err error) {
	if form.Config != nil {
		if url, ok := form.Config["url"]; ok {
			w.URL = url
		}
		if ct, ok := form.Config["content_type"]; ok {
			if !webhook_model.IsValidHookContentType(ct) {
				return http.StatusUnprocessableEntity, "", errors.New("invalid content type")
			}
			w.ContentType = webhook_model.ToHookContentType(ct)
		}

		if w.Type == webhook_module.SLACK {
			if channel, ok := form.Config["channel"]; ok {
				meta, err := json.Marshal(&SlackMeta{
					Channel:  channel,
					Username: form.Config["username"],
					IconURL:  form.Config["icon_url"],
					Color:    form.Config["color"],
				})
				if err != nil {
					return http.StatusInternalServerError, "slack: JSON marshal failed", err
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

	err = w.SetHeaderAuthorization(form.AuthorizationHeader)
	if err != nil {
		return http.StatusInternalServerError, "SetHeaderAuthorization", err
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
		return http.StatusInternalServerError, "UpdateEvent", err
	}

	if form.Active != nil {
		w.IsActive = *form.Active
	}

	if err := webhook_model.UpdateWebhook(w); err != nil {
		return http.StatusInternalServerError, "UpdateWebhook", err
	}
	return http.StatusOK, "", nil
}
