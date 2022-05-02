// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"code.gitea.io/gitea/models/perm"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	webhook_service "code.gitea.io/gitea/services/webhook"
)

const (
	tplHooks        base.TplName = "repo/settings/webhook/base"
	tplHookNew      base.TplName = "repo/settings/webhook/new"
	tplOrgHookNew   base.TplName = "org/settings/hook_new"
	tplAdminHookNew base.TplName = "admin/hook_new"
)

// Webhooks render web hooks list page
func Webhooks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.hooks")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["BaseLink"] = ctx.Repo.RepoLink + "/settings/hooks"
	ctx.Data["BaseLinkNew"] = ctx.Repo.RepoLink + "/settings/hooks"
	ctx.Data["Description"] = ctx.Tr("repo.settings.hooks_desc", "https://docs.gitea.io/en-us/webhooks/")

	ws, err := webhook.ListWebhooksByOpts(&webhook.ListWebhookOptions{RepoID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.ServerError("GetWebhooksByRepoID", err)
		return
	}
	ctx.Data["Webhooks"] = ws

	ctx.HTML(http.StatusOK, tplHooks)
}

type orgRepoCtx struct {
	OrgID           int64
	RepoID          int64
	IsAdmin         bool
	IsSystemWebhook bool
	Link            string
	LinkNew         string
	NewTemplate     base.TplName
}

// getOrgRepoCtx determines whether this is a repo, organization, or admin (both default and system) context.
func getOrgRepoCtx(ctx *context.Context) (*orgRepoCtx, error) {
	if len(ctx.Repo.RepoLink) > 0 {
		return &orgRepoCtx{
			RepoID:      ctx.Repo.Repository.ID,
			Link:        path.Join(ctx.Repo.RepoLink, "settings/hooks"),
			LinkNew:     path.Join(ctx.Repo.RepoLink, "settings/hooks"),
			NewTemplate: tplHookNew,
		}, nil
	}

	if len(ctx.Org.OrgLink) > 0 {
		return &orgRepoCtx{
			OrgID:       ctx.Org.Organization.ID,
			Link:        path.Join(ctx.Org.OrgLink, "settings/hooks"),
			LinkNew:     path.Join(ctx.Org.OrgLink, "settings/hooks"),
			NewTemplate: tplOrgHookNew,
		}, nil
	}

	if ctx.Doer.IsAdmin {
		// Are we looking at default webhooks?
		if ctx.Params(":configType") == "default-hooks" {
			return &orgRepoCtx{
				IsAdmin:     true,
				Link:        path.Join(setting.AppSubURL, "/admin/hooks"),
				LinkNew:     path.Join(setting.AppSubURL, "/admin/default-hooks"),
				NewTemplate: tplAdminHookNew,
			}, nil
		}

		// Must be system webhooks instead
		return &orgRepoCtx{
			IsAdmin:         true,
			IsSystemWebhook: true,
			Link:            path.Join(setting.AppSubURL, "/admin/hooks"),
			LinkNew:         path.Join(setting.AppSubURL, "/admin/system-hooks"),
			NewTemplate:     tplAdminHookNew,
		}, nil
	}

	return nil, errors.New("unable to set OrgRepo context")
}

func checkHookType(ctx *context.Context) string {
	hookType := strings.ToLower(ctx.Params(":type"))
	if !util.IsStringInSlice(hookType, setting.Webhook.Types, true) {
		ctx.NotFound("checkHookType", nil)
		return ""
	}
	return hookType
}

// WebhooksNew render creating webhook page
func WebhooksNew(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.add_webhook")
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook.HookEvent{}}

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if orCtx.IsAdmin && orCtx.IsSystemWebhook {
		ctx.Data["PageIsAdminSystemHooks"] = true
		ctx.Data["PageIsAdminSystemHooksNew"] = true
	} else if orCtx.IsAdmin {
		ctx.Data["PageIsAdminDefaultHooks"] = true
		ctx.Data["PageIsAdminDefaultHooksNew"] = true
	} else {
		ctx.Data["PageIsSettingsHooks"] = true
		ctx.Data["PageIsSettingsHooksNew"] = true
	}

	hookType := checkHookType(ctx)
	ctx.Data["HookType"] = hookType
	if ctx.Written() {
		return
	}
	if hookType == "discord" {
		ctx.Data["DiscordHook"] = map[string]interface{}{
			"Username": "Gitea",
		}
	}
	ctx.Data["BaseLink"] = orCtx.LinkNew

	ctx.HTML(http.StatusOK, orCtx.NewTemplate)
}

// ParseHookEvent convert web form content to webhook.HookEvent
func ParseHookEvent(form forms.WebhookForm) *webhook.HookEvent {
	return &webhook.HookEvent{
		PushOnly:       form.PushOnly(),
		SendEverything: form.SendEverything(),
		ChooseEvents:   form.ChooseEvents(),
		HookEvents: webhook.HookEvents{
			Create:               form.Create,
			Delete:               form.Delete,
			Fork:                 form.Fork,
			Issues:               form.Issues,
			IssueAssign:          form.IssueAssign,
			IssueLabel:           form.IssueLabel,
			IssueMilestone:       form.IssueMilestone,
			IssueComment:         form.IssueComment,
			Release:              form.Release,
			Push:                 form.Push,
			PullRequest:          form.PullRequest,
			PullRequestAssign:    form.PullRequestAssign,
			PullRequestLabel:     form.PullRequestLabel,
			PullRequestMilestone: form.PullRequestMilestone,
			PullRequestComment:   form.PullRequestComment,
			PullRequestReview:    form.PullRequestReview,
			PullRequestSync:      form.PullRequestSync,
			Repository:           form.Repository,
			Package:              form.Package,
		},
		BranchFilter: form.BranchFilter,
	}
}

// GiteaHooksNewPost response for creating Gitea webhook
func GiteaHooksNewPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewWebhookForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings.add_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook.HookEvent{}}
	ctx.Data["HookType"] = webhook.GITEA

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}
	ctx.Data["BaseLink"] = orCtx.LinkNew

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, orCtx.NewTemplate)
		return
	}

	contentType := webhook.ContentTypeJSON
	if webhook.HookContentType(form.ContentType) == webhook.ContentTypeForm {
		contentType = webhook.ContentTypeForm
	}

	w := &webhook.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		HTTPMethod:      form.HTTPMethod,
		ContentType:     contentType,
		Secret:          form.Secret,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		Type:            webhook.GITEA,
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

// GogsHooksNewPost response for creating webhook
func GogsHooksNewPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewGogshookForm)
	newGogsWebhookPost(ctx, *form, webhook.GOGS)
}

// newGogsWebhookPost response for creating gogs hook
func newGogsWebhookPost(ctx *context.Context, form forms.NewGogshookForm, kind webhook.HookType) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.add_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook.HookEvent{}}
	ctx.Data["HookType"] = webhook.GOGS

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}
	ctx.Data["BaseLink"] = orCtx.LinkNew

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, orCtx.NewTemplate)
		return
	}

	contentType := webhook.ContentTypeJSON
	if webhook.HookContentType(form.ContentType) == webhook.ContentTypeForm {
		contentType = webhook.ContentTypeForm
	}

	w := &webhook.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		ContentType:     contentType,
		Secret:          form.Secret,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		Type:            kind,
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

// DiscordHooksNewPost response for creating discord hook
func DiscordHooksNewPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewDiscordHookForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook.HookEvent{}}
	ctx.Data["HookType"] = webhook.DISCORD

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, orCtx.NewTemplate)
		return
	}

	meta, err := json.Marshal(&webhook_service.DiscordMeta{
		Username: form.Username,
		IconURL:  form.IconURL,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w := &webhook.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		ContentType:     webhook.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		Type:            webhook.DISCORD,
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

// DingtalkHooksNewPost response for creating dingtalk hook
func DingtalkHooksNewPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewDingtalkHookForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook.HookEvent{}}
	ctx.Data["HookType"] = webhook.DINGTALK

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, orCtx.NewTemplate)
		return
	}

	w := &webhook.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		ContentType:     webhook.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		Type:            webhook.DINGTALK,
		Meta:            "",
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

// TelegramHooksNewPost response for creating telegram hook
func TelegramHooksNewPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewTelegramHookForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook.HookEvent{}}
	ctx.Data["HookType"] = webhook.TELEGRAM

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, orCtx.NewTemplate)
		return
	}

	meta, err := json.Marshal(&webhook_service.TelegramMeta{
		BotToken: form.BotToken,
		ChatID:   form.ChatID,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w := &webhook.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=%s", url.PathEscape(form.BotToken), url.QueryEscape(form.ChatID)),
		ContentType:     webhook.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		Type:            webhook.TELEGRAM,
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

// MatrixHooksNewPost response for creating a Matrix hook
func MatrixHooksNewPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewMatrixHookForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook.HookEvent{}}
	ctx.Data["HookType"] = webhook.MATRIX

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, orCtx.NewTemplate)
		return
	}

	meta, err := json.Marshal(&webhook_service.MatrixMeta{
		HomeserverURL: form.HomeserverURL,
		Room:          form.RoomID,
		AccessToken:   form.AccessToken,
		MessageType:   form.MessageType,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w := &webhook.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             fmt.Sprintf("%s/_matrix/client/r0/rooms/%s/send/m.room.message", form.HomeserverURL, url.PathEscape(form.RoomID)),
		ContentType:     webhook.ContentTypeJSON,
		HTTPMethod:      "PUT",
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		Type:            webhook.MATRIX,
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

// MSTeamsHooksNewPost response for creating MS Teams hook
func MSTeamsHooksNewPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewMSTeamsHookForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook.HookEvent{}}
	ctx.Data["HookType"] = webhook.MSTEAMS

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, orCtx.NewTemplate)
		return
	}

	w := &webhook.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		ContentType:     webhook.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		Type:            webhook.MSTEAMS,
		Meta:            "",
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

// SlackHooksNewPost response for creating slack hook
func SlackHooksNewPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewSlackHookForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook.HookEvent{}}
	ctx.Data["HookType"] = webhook.SLACK

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, orCtx.NewTemplate)
		return
	}

	if form.HasInvalidChannel() {
		ctx.Flash.Error(ctx.Tr("repo.settings.add_webhook.invalid_channel_name"))
		ctx.Redirect(orCtx.LinkNew + "/slack/new")
		return
	}

	meta, err := json.Marshal(&webhook_service.SlackMeta{
		Channel:  strings.TrimSpace(form.Channel),
		Username: form.Username,
		IconURL:  form.IconURL,
		Color:    form.Color,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w := &webhook.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		ContentType:     webhook.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		Type:            webhook.SLACK,
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

// FeishuHooksNewPost response for creating feishu hook
func FeishuHooksNewPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewFeishuHookForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook.HookEvent{}}
	ctx.Data["HookType"] = webhook.FEISHU

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, orCtx.NewTemplate)
		return
	}

	w := &webhook.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		ContentType:     webhook.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		Type:            webhook.FEISHU,
		Meta:            "",
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

// WechatworkHooksNewPost response for creating wechatwork hook
func WechatworkHooksNewPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewWechatWorkHookForm)

	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook.HookEvent{}}
	ctx.Data["HookType"] = webhook.WECHATWORK

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, orCtx.NewTemplate)
		return
	}

	w := &webhook.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		ContentType:     webhook.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		Type:            webhook.WECHATWORK,
		Meta:            "",
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

// PackagistHooksNewPost response for creating packagist hook
func PackagistHooksNewPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewPackagistHookForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook.HookEvent{}}
	ctx.Data["HookType"] = webhook.PACKAGIST

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, orCtx.NewTemplate)
		return
	}

	meta, err := json.Marshal(&webhook_service.PackagistMeta{
		Username:   form.Username,
		APIToken:   form.APIToken,
		PackageURL: form.PackageURL,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w := &webhook.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             fmt.Sprintf("https://packagist.org/api/update-package?username=%s&apiToken=%s", url.QueryEscape(form.Username), url.QueryEscape(form.APIToken)),
		ContentType:     webhook.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		Type:            webhook.PACKAGIST,
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

func checkWebhook(ctx *context.Context) (*orgRepoCtx, *webhook.Webhook) {
	ctx.Data["RequireHighlightJS"] = true

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return nil, nil
	}
	ctx.Data["BaseLink"] = orCtx.Link

	var w *webhook.Webhook
	if orCtx.RepoID > 0 {
		w, err = webhook.GetWebhookByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	} else if orCtx.OrgID > 0 {
		w, err = webhook.GetWebhookByOrgID(ctx.Org.Organization.ID, ctx.ParamsInt64(":id"))
	} else if orCtx.IsAdmin {
		w, err = webhook.GetSystemOrDefaultWebhook(ctx.ParamsInt64(":id"))
	}
	if err != nil || w == nil {
		if webhook.IsErrWebhookNotExist(err) {
			ctx.NotFound("GetWebhookByID", nil)
		} else {
			ctx.ServerError("GetWebhookByID", err)
		}
		return nil, nil
	}

	ctx.Data["HookType"] = w.Type
	switch w.Type {
	case webhook.SLACK:
		ctx.Data["SlackHook"] = webhook_service.GetSlackHook(w)
	case webhook.DISCORD:
		ctx.Data["DiscordHook"] = webhook_service.GetDiscordHook(w)
	case webhook.TELEGRAM:
		ctx.Data["TelegramHook"] = webhook_service.GetTelegramHook(w)
	case webhook.MATRIX:
		ctx.Data["MatrixHook"] = webhook_service.GetMatrixHook(w)
	case webhook.PACKAGIST:
		ctx.Data["PackagistHook"] = webhook_service.GetPackagistHook(w)
	}

	ctx.Data["History"], err = w.History(1)
	if err != nil {
		ctx.ServerError("History", err)
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

	ctx.HTML(http.StatusOK, orCtx.NewTemplate)
}

// WebHooksEditPost response for editing web hook
func WebHooksEditPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewWebhookForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings.update_webhook")
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

	contentType := webhook.ContentTypeJSON
	if webhook.HookContentType(form.ContentType) == webhook.ContentTypeForm {
		contentType = webhook.ContentTypeForm
	}

	w.URL = form.PayloadURL
	w.ContentType = contentType
	w.Secret = form.Secret
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	w.HTTPMethod = form.HTTPMethod
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := webhook.UpdateWebhook(w); err != nil {
		ctx.ServerError("WebHooksEditPost", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// GogsHooksEditPost response for editing gogs hook
func GogsHooksEditPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewGogshookForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings.update_webhook")
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

	contentType := webhook.ContentTypeJSON
	if webhook.HookContentType(form.ContentType) == webhook.ContentTypeForm {
		contentType = webhook.ContentTypeForm
	}

	w.URL = form.PayloadURL
	w.ContentType = contentType
	w.Secret = form.Secret
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := webhook.UpdateWebhook(w); err != nil {
		ctx.ServerError("GogsHooksEditPost", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// SlackHooksEditPost response for editing slack hook
func SlackHooksEditPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewSlackHookForm)
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

	if form.HasInvalidChannel() {
		ctx.Flash.Error(ctx.Tr("repo.settings.add_webhook.invalid_channel_name"))
		ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
		return
	}

	meta, err := json.Marshal(&webhook_service.SlackMeta{
		Channel:  strings.TrimSpace(form.Channel),
		Username: form.Username,
		IconURL:  form.IconURL,
		Color:    form.Color,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w.URL = form.PayloadURL
	w.Meta = string(meta)
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

// DiscordHooksEditPost response for editing discord hook
func DiscordHooksEditPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewDiscordHookForm)
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

	meta, err := json.Marshal(&webhook_service.DiscordMeta{
		Username: form.Username,
		IconURL:  form.IconURL,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w.URL = form.PayloadURL
	w.Meta = string(meta)
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

// DingtalkHooksEditPost response for editing discord hook
func DingtalkHooksEditPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewDingtalkHookForm)
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

	w.URL = form.PayloadURL
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

// TelegramHooksEditPost response for editing discord hook
func TelegramHooksEditPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewTelegramHookForm)
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

	meta, err := json.Marshal(&webhook_service.TelegramMeta{
		BotToken: form.BotToken,
		ChatID:   form.ChatID,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}
	w.Meta = string(meta)
	w.URL = fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=%s", url.PathEscape(form.BotToken), url.QueryEscape(form.ChatID))
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

// MatrixHooksEditPost response for editing a Matrix hook
func MatrixHooksEditPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewMatrixHookForm)
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

	meta, err := json.Marshal(&webhook_service.MatrixMeta{
		HomeserverURL: form.HomeserverURL,
		Room:          form.RoomID,
		AccessToken:   form.AccessToken,
		MessageType:   form.MessageType,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}
	w.Meta = string(meta)
	w.URL = fmt.Sprintf("%s/_matrix/client/r0/rooms/%s/send/m.room.message", form.HomeserverURL, url.PathEscape(form.RoomID))

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

// MSTeamsHooksEditPost response for editing MS Teams hook
func MSTeamsHooksEditPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewMSTeamsHookForm)
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

	w.URL = form.PayloadURL
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

// FeishuHooksEditPost response for editing feishu hook
func FeishuHooksEditPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewFeishuHookForm)
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

	w.URL = form.PayloadURL
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

// WechatworkHooksEditPost response for editing wechatwork hook
func WechatworkHooksEditPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewWechatWorkHookForm)
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

	w.URL = form.PayloadURL
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

// PackagistHooksEditPost response for editing packagist hook
func PackagistHooksEditPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewPackagistHookForm)
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

	meta, err := json.Marshal(&webhook_service.PackagistMeta{
		Username:   form.Username,
		APIToken:   form.APIToken,
		PackageURL: form.PackageURL,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w.Meta = string(meta)
	w.URL = fmt.Sprintf("https://packagist.org/api/update-package?username=%s&apiToken=%s", url.QueryEscape(form.Username), url.QueryEscape(form.APIToken))
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

// TestWebhook test if web hook is work fine
func TestWebhook(ctx *context.Context) {
	hookID := ctx.ParamsInt64(":id")
	w, err := webhook.GetWebhookByRepoID(ctx.Repo.Repository.ID, hookID)
	if err != nil {
		ctx.Flash.Error("GetWebhookByID: " + err.Error())
		ctx.Status(http.StatusInternalServerError)
		return
	}

	// Grab latest commit or fake one if it's empty repository.
	commit := ctx.Repo.Commit
	if commit == nil {
		ghost := user_model.NewGhostUser()
		commit = &git.Commit{
			ID:            git.MustIDFromString(git.EmptySHA),
			Author:        ghost.NewGitSig(),
			Committer:     ghost.NewGitSig(),
			CommitMessage: "This is a fake commit",
		}
	}

	apiUser := convert.ToUserWithAccessMode(ctx.Doer, perm.AccessModeNone)

	apiCommit := &api.PayloadCommit{
		ID:      commit.ID.String(),
		Message: commit.Message(),
		URL:     ctx.Repo.Repository.HTMLURL() + "/commit/" + url.PathEscape(commit.ID.String()),
		Author: &api.PayloadUser{
			Name:  commit.Author.Name,
			Email: commit.Author.Email,
		},
		Committer: &api.PayloadUser{
			Name:  commit.Committer.Name,
			Email: commit.Committer.Email,
		},
	}

	p := &api.PushPayload{
		Ref:        git.BranchPrefix + ctx.Repo.Repository.DefaultBranch,
		Before:     commit.ID.String(),
		After:      commit.ID.String(),
		Commits:    []*api.PayloadCommit{apiCommit},
		HeadCommit: apiCommit,
		Repo:       convert.ToRepo(ctx.Repo.Repository, perm.AccessModeNone),
		Pusher:     apiUser,
		Sender:     apiUser,
	}
	if err := webhook_service.PrepareWebhook(w, ctx.Repo.Repository, webhook.HookEventPush, p); err != nil {
		ctx.Flash.Error("PrepareWebhook: " + err.Error())
		ctx.Status(http.StatusInternalServerError)
	} else {
		ctx.Flash.Info(ctx.Tr("repo.settings.webhook.delivery.success"))
		ctx.Status(http.StatusOK)
	}
}

// ReplayWebhook replays a webhook
func ReplayWebhook(ctx *context.Context) {
	hookTaskUUID := ctx.Params(":uuid")

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}

	if err := webhook_service.ReplayHookTask(w, hookTaskUUID); err != nil {
		if webhook.IsErrHookTaskNotExist(err) {
			ctx.NotFound("ReplayHookTask", nil)
		} else {
			ctx.ServerError("ReplayHookTask", err)
		}
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.webhook.delivery.success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// DeleteWebhook delete a webhook
func DeleteWebhook(ctx *context.Context) {
	if err := webhook.DeleteWebhookByRepoID(ctx.Repo.Repository.ID, ctx.FormInt64("id")); err != nil {
		ctx.Flash.Error("DeleteWebhookByRepoID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.webhook_deletion_success"))
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/settings/hooks",
	})
}
