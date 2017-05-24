// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Unknwon/com"

	"code.gitea.io/git"
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplHooks      base.TplName = "repo/settings/hooks"
	tplHookNew    base.TplName = "repo/settings/hook_new"
	tplOrgHookNew base.TplName = "org/settings/hook_new"
)

// Webhooks render web hooks list page
func Webhooks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.hooks")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["BaseLink"] = ctx.Repo.RepoLink
	ctx.Data["Description"] = ctx.Tr("repo.settings.hooks_desc", "https://godoc.org/code.gitea.io/sdk/gitea")

	ws, err := models.GetWebhooksByRepoID(ctx.Repo.Repository.ID)
	if err != nil {
		ctx.Handle(500, "GetWebhooksByRepoID", err)
		return
	}
	ctx.Data["Webhooks"] = ws

	ctx.HTML(200, tplHooks)
}

type orgRepoCtx struct {
	OrgID       int64
	RepoID      int64
	Link        string
	NewTemplate base.TplName
}

// getOrgRepoCtx determines whether this is a repo context or organization context.
func getOrgRepoCtx(ctx *context.Context) (*orgRepoCtx, error) {
	if len(ctx.Repo.RepoLink) > 0 {
		return &orgRepoCtx{
			RepoID:      ctx.Repo.Repository.ID,
			Link:        ctx.Repo.RepoLink,
			NewTemplate: tplHookNew,
		}, nil
	}

	if len(ctx.Org.OrgLink) > 0 {
		return &orgRepoCtx{
			OrgID:       ctx.Org.Organization.ID,
			Link:        ctx.Org.OrgLink,
			NewTemplate: tplOrgHookNew,
		}, nil
	}

	return nil, errors.New("Unable to set OrgRepo context")
}

func checkHookType(ctx *context.Context) string {
	hookType := strings.ToLower(ctx.Params(":type"))
	if !com.IsSliceContainsStr(setting.Webhook.Types, hookType) {
		ctx.Handle(404, "checkHookType", nil)
		return ""
	}
	return hookType
}

// WebhooksNew render creating webhook page
func WebhooksNew(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.add_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.Handle(500, "getOrgRepoCtx", err)
		return
	}

	ctx.Data["HookType"] = checkHookType(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["BaseLink"] = orCtx.Link

	ctx.HTML(200, orCtx.NewTemplate)
}

// ParseHookEvent convert web form content to models.HookEvent
func ParseHookEvent(form auth.WebhookForm) *models.HookEvent {
	return &models.HookEvent{
		PushOnly:       form.PushOnly(),
		SendEverything: form.SendEverything(),
		ChooseEvents:   form.ChooseEvents(),
		HookEvents: models.HookEvents{
			Create:      form.Create,
			Push:        form.Push,
			PullRequest: form.PullRequest,
		},
	}
}

// WebHooksNewPost response for creating webhook
func WebHooksNewPost(ctx *context.Context, form auth.NewWebhookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.add_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}
	ctx.Data["HookType"] = "gitea"

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.Handle(500, "getOrgRepoCtx", err)
		return
	}
	ctx.Data["BaseLink"] = orCtx.Link

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	contentType := models.ContentTypeJSON
	if models.HookContentType(form.ContentType) == models.ContentTypeForm {
		contentType = models.ContentTypeForm
	}

	w := &models.Webhook{
		RepoID:       orCtx.RepoID,
		URL:          form.PayloadURL,
		ContentType:  contentType,
		Secret:       form.Secret,
		HookEvent:    ParseHookEvent(form.WebhookForm),
		IsActive:     form.Active,
		HookTaskType: models.GITEA,
		OrgID:        orCtx.OrgID,
	}
	if err := w.UpdateEvent(); err != nil {
		ctx.Handle(500, "UpdateEvent", err)
		return
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.Handle(500, "CreateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_hook_success"))
	ctx.Redirect(orCtx.Link + "/settings/hooks")
}

// GogsHooksNewPost response for creating webhook
func GogsHooksNewPost(ctx *context.Context, form auth.NewWebhookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.add_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}
	ctx.Data["HookType"] = "gogs"

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.Handle(500, "getOrgRepoCtx", err)
		return
	}
	ctx.Data["BaseLink"] = orCtx.Link

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	contentType := models.ContentTypeJSON
	if models.HookContentType(form.ContentType) == models.ContentTypeForm {
		contentType = models.ContentTypeForm
	}

	w := &models.Webhook{
		RepoID:       orCtx.RepoID,
		URL:          form.PayloadURL,
		ContentType:  contentType,
		Secret:       form.Secret,
		HookEvent:    ParseHookEvent(form.WebhookForm),
		IsActive:     form.Active,
		HookTaskType: models.GITEA,
		OrgID:        orCtx.OrgID,
	}
	if err := w.UpdateEvent(); err != nil {
		ctx.Handle(500, "UpdateEvent", err)
		return
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.Handle(500, "CreateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_hook_success"))
	ctx.Redirect(orCtx.Link + "/settings/hooks")
}

// SlackHooksNewPost response for creating slack hook
func SlackHooksNewPost(ctx *context.Context, form auth.NewSlackHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.Handle(500, "getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	meta, err := json.Marshal(&models.SlackMeta{
		Channel:  form.Channel,
		Username: form.Username,
		IconURL:  form.IconURL,
		Color:    form.Color,
	})
	if err != nil {
		ctx.Handle(500, "Marshal", err)
		return
	}

	w := &models.Webhook{
		RepoID:       orCtx.RepoID,
		URL:          form.PayloadURL,
		ContentType:  models.ContentTypeJSON,
		HookEvent:    ParseHookEvent(form.WebhookForm),
		IsActive:     form.Active,
		HookTaskType: models.SLACK,
		Meta:         string(meta),
		OrgID:        orCtx.OrgID,
	}
	if err := w.UpdateEvent(); err != nil {
		ctx.Handle(500, "UpdateEvent", err)
		return
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.Handle(500, "CreateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_hook_success"))
	ctx.Redirect(orCtx.Link + "/settings/hooks")
}

func checkWebhook(ctx *context.Context) (*orgRepoCtx, *models.Webhook) {
	ctx.Data["RequireHighlightJS"] = true

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.Handle(500, "getOrgRepoCtx", err)
		return nil, nil
	}
	ctx.Data["BaseLink"] = orCtx.Link

	var w *models.Webhook
	if orCtx.RepoID > 0 {
		w, err = models.GetWebhookByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	} else {
		w, err = models.GetWebhookByOrgID(ctx.Org.Organization.ID, ctx.ParamsInt64(":id"))
	}
	if err != nil {
		if models.IsErrWebhookNotExist(err) {
			ctx.Handle(404, "GetWebhookByID", nil)
		} else {
			ctx.Handle(500, "GetWebhookByID", err)
		}
		return nil, nil
	}

	switch w.HookTaskType {
	case models.SLACK:
		ctx.Data["SlackHook"] = w.GetSlackHook()
		ctx.Data["HookType"] = "slack"
	case models.GOGS:
		ctx.Data["HookType"] = "gogs"
	default:
		ctx.Data["HookType"] = "gitea"
	}

	ctx.Data["History"], err = w.History(1)
	if err != nil {
		ctx.Handle(500, "History", err)
	}
	return orCtx, w
}

// WebHooksEdit render editing web hook page
func WebHooksEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.update_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	ctx.HTML(200, orCtx.NewTemplate)
}

// WebHooksEditPost response for editing web hook
func WebHooksEditPost(ctx *context.Context, form auth.NewWebhookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.update_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	contentType := models.ContentTypeJSON
	if models.HookContentType(form.ContentType) == models.ContentTypeForm {
		contentType = models.ContentTypeForm
	}

	w.URL = form.PayloadURL
	w.ContentType = contentType
	w.Secret = form.Secret
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	if err := w.UpdateEvent(); err != nil {
		ctx.Handle(500, "UpdateEvent", err)
		return
	} else if err := models.UpdateWebhook(w); err != nil {
		ctx.Handle(500, "WebHooksEditPost", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/settings/hooks/%d", orCtx.Link, w.ID))
}

// GogsHooksEditPost response for editing gogs hook
func GogsHooksEditPost(ctx *context.Context, form auth.NewWebhookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.update_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	contentType := models.ContentTypeJSON
	if models.HookContentType(form.ContentType) == models.ContentTypeForm {
		contentType = models.ContentTypeForm
	}

	w.URL = form.PayloadURL
	w.ContentType = contentType
	w.Secret = form.Secret
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	if err := w.UpdateEvent(); err != nil {
		ctx.Handle(500, "UpdateEvent", err)
		return
	} else if err := models.UpdateWebhook(w); err != nil {
		ctx.Handle(500, "GogsHooksEditPost", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/settings/hooks/%d", orCtx.Link, w.ID))
}

// SlackHooksEditPost response for editing slack hook
func SlackHooksEditPost(ctx *context.Context, form auth.NewSlackHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	meta, err := json.Marshal(&models.SlackMeta{
		Channel:  form.Channel,
		Username: form.Username,
		IconURL:  form.IconURL,
		Color:    form.Color,
	})
	if err != nil {
		ctx.Handle(500, "Marshal", err)
		return
	}

	w.URL = form.PayloadURL
	w.Meta = string(meta)
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	if err := w.UpdateEvent(); err != nil {
		ctx.Handle(500, "UpdateEvent", err)
		return
	} else if err := models.UpdateWebhook(w); err != nil {
		ctx.Handle(500, "UpdateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/settings/hooks/%d", orCtx.Link, w.ID))
}

// TestWebhook test if web hook is work fine
func TestWebhook(ctx *context.Context) {
	// Grab latest commit or fake one if it's empty repository.
	commit := ctx.Repo.Commit
	if commit == nil {
		ghost := models.NewGhostUser()
		commit = &git.Commit{
			ID:            git.MustIDFromString(git.EmptySHA),
			Author:        ghost.NewGitSig(),
			Committer:     ghost.NewGitSig(),
			CommitMessage: "This is a fake commit",
		}
	}

	apiUser := ctx.User.APIFormat()
	p := &api.PushPayload{
		Ref:    git.BranchPrefix + ctx.Repo.Repository.DefaultBranch,
		Before: commit.ID.String(),
		After:  commit.ID.String(),
		Commits: []*api.PayloadCommit{
			{
				ID:      commit.ID.String(),
				Message: commit.Message(),
				URL:     ctx.Repo.Repository.HTMLURL() + "/commit/" + commit.ID.String(),
				Author: &api.PayloadUser{
					Name:  commit.Author.Name,
					Email: commit.Author.Email,
				},
				Committer: &api.PayloadUser{
					Name:  commit.Committer.Name,
					Email: commit.Committer.Email,
				},
			},
		},
		Repo:   ctx.Repo.Repository.APIFormat(models.AccessModeNone),
		Pusher: apiUser,
		Sender: apiUser,
	}
	if err := models.PrepareWebhooks(ctx.Repo.Repository, models.HookEventPush, p); err != nil {
		ctx.Flash.Error("PrepareWebhooks: " + err.Error())
		ctx.Status(500)
	} else {
		go models.HookQueue.Add(ctx.Repo.Repository.ID)
		ctx.Flash.Info(ctx.Tr("repo.settings.webhook.test_delivery_success"))
		ctx.Status(200)
	}
}

// DeleteWebhook delete a webhook
func DeleteWebhook(ctx *context.Context) {
	if err := models.DeleteWebhookByRepoID(ctx.Repo.Repository.ID, ctx.QueryInt64("id")); err != nil {
		ctx.Flash.Error("DeleteWebhookByRepoID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.webhook_deletion_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/settings/hooks",
	})
}
