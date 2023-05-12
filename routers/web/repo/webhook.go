// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/forms"
	webhook_service "code.gitea.io/gitea/services/webhook"
)

const (
	tplHooks        base.TplName = "repo/settings/webhook/base"
	tplHookNew      base.TplName = "repo/settings/webhook/new"
	tplOrgHookNew   base.TplName = "org/settings/hook_new"
	tplUserHookNew  base.TplName = "user/settings/hook_new"
	tplAdminHookNew base.TplName = "admin/hook_new"
)

// Webhooks render web hooks list page
func Webhooks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.hooks")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["BaseLink"] = ctx.Repo.RepoLink + "/settings/hooks"
	ctx.Data["BaseLinkNew"] = ctx.Repo.RepoLink + "/settings/hooks"
	ctx.Data["Description"] = ctx.Tr("repo.settings.hooks_desc", "https://docs.gitea.io/en-us/webhooks/")

	ws, err := webhook.ListWebhooksByOpts(ctx, &webhook.ListWebhookOptions{RepoID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.ServerError("GetWebhooksByRepoID", err)
		return
	}
	ctx.Data["Webhooks"] = ws

	ctx.HTML(http.StatusOK, tplHooks)
}

type ownerRepoCtx struct {
	OwnerID         int64
	RepoID          int64
	IsAdmin         bool
	IsSystemWebhook bool
	Link            string
	LinkNew         string
	NewTemplate     base.TplName
}

// getOwnerRepoCtx determines whether this is a repo, owner, or admin (both default and system) context.
func getOwnerRepoCtx(ctx *context.Context) (*ownerRepoCtx, error) {
	if ctx.Data["PageIsRepoSettings"] == true {
		return &ownerRepoCtx{
			RepoID:      ctx.Repo.Repository.ID,
			Link:        path.Join(ctx.Repo.RepoLink, "settings/hooks"),
			LinkNew:     path.Join(ctx.Repo.RepoLink, "settings/hooks"),
			NewTemplate: tplHookNew,
		}, nil
	}

	if ctx.Data["PageIsOrgSettings"] == true {
		return &ownerRepoCtx{
			OwnerID:     ctx.ContextUser.ID,
			Link:        path.Join(ctx.Org.OrgLink, "settings/hooks"),
			LinkNew:     path.Join(ctx.Org.OrgLink, "settings/hooks"),
			NewTemplate: tplOrgHookNew,
		}, nil
	}

	if ctx.Data["PageIsUserSettings"] == true {
		return &ownerRepoCtx{
			OwnerID:     ctx.Doer.ID,
			Link:        path.Join(setting.AppSubURL, "/user/settings/hooks"),
			LinkNew:     path.Join(setting.AppSubURL, "/user/settings/hooks"),
			NewTemplate: tplUserHookNew,
		}, nil
	}

	if ctx.Data["PageIsAdmin"] == true {
		return &ownerRepoCtx{
			IsAdmin:         true,
			IsSystemWebhook: ctx.Params(":configType") == "system-hooks",
			Link:            path.Join(setting.AppSubURL, "/admin/hooks"),
			LinkNew:         path.Join(setting.AppSubURL, "/admin/", ctx.Params(":configType")),
			NewTemplate:     tplAdminHookNew,
		}, nil
	}

	return nil, errors.New("unable to set OwnerRepo context")
}

func checkHookType(ctx *context.Context) string {
	hookType := strings.ToLower(ctx.Params(":type"))
	if !util.SliceContainsString(setting.Webhook.Types, hookType, true) {
		ctx.NotFound("checkHookType", nil)
		return ""
	}
	return hookType
}

// WebhooksNew render creating webhook page
func WebhooksNew(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.add_webhook")
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook_module.HookEvent{}}

	orCtx, err := getOwnerRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOwnerRepoCtx", err)
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
func ParseHookEvent(form forms.WebhookForm) *webhook_module.HookEvent {
	return &webhook_module.HookEvent{
		PushOnly:       form.PushOnly(),
		SendEverything: form.SendEverything(),
		ChooseEvents:   form.ChooseEvents(),
		HookEvents: webhook_module.HookEvents{
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
			Wiki:                 form.Wiki,
			Repository:           form.Repository,
			Package:              form.Package,
		},
		BranchFilter: form.BranchFilter,
	}
}

type webhookParams struct {
	// Type should be imported from webhook package (webhook.XXX)
	Type string

	URL         string
	ContentType webhook.HookContentType
	Secret      string
	HTTPMethod  string
	WebhookForm forms.WebhookForm
	Meta        interface{}
}

func createWebhook(ctx *context.Context, params webhookParams) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.add_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = webhook.Webhook{HookEvent: &webhook_module.HookEvent{}}
	ctx.Data["HookType"] = params.Type

	orCtx, err := getOwnerRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOwnerRepoCtx", err)
		return
	}
	ctx.Data["BaseLink"] = orCtx.LinkNew

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, orCtx.NewTemplate)
		return
	}

	var meta []byte
	if params.Meta != nil {
		meta, err = json.Marshal(params.Meta)
		if err != nil {
			ctx.ServerError("Marshal", err)
			return
		}
	}

	w := &webhook.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             params.URL,
		HTTPMethod:      params.HTTPMethod,
		ContentType:     params.ContentType,
		Secret:          params.Secret,
		HookEvent:       ParseHookEvent(params.WebhookForm),
		IsActive:        params.WebhookForm.Active,
		Type:            params.Type,
		Meta:            string(meta),
		OwnerID:         orCtx.OwnerID,
		IsSystemWebhook: orCtx.IsSystemWebhook,
	}
	err = w.SetHeaderAuthorization(params.WebhookForm.AuthorizationHeader)
	if err != nil {
		ctx.ServerError("SetHeaderAuthorization", err)
		return
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

func editWebhook(ctx *context.Context, params webhookParams) {
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

	var meta []byte
	var err error
	if params.Meta != nil {
		meta, err = json.Marshal(params.Meta)
		if err != nil {
			ctx.ServerError("Marshal", err)
			return
		}
	}

	w.URL = params.URL
	w.ContentType = params.ContentType
	w.Secret = params.Secret
	w.HookEvent = ParseHookEvent(params.WebhookForm)
	w.IsActive = params.WebhookForm.Active
	w.HTTPMethod = params.HTTPMethod
	w.Meta = string(meta)

	err = w.SetHeaderAuthorization(params.WebhookForm.AuthorizationHeader)
	if err != nil {
		ctx.ServerError("SetHeaderAuthorization", err)
		return
	}

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

// GiteaHooksNewPost response for creating Gitea webhook
func GiteaHooksNewPost(ctx *context.Context) {
	createWebhook(ctx, giteaHookParams(ctx))
}

// GiteaHooksEditPost response for editing Gitea webhook
func GiteaHooksEditPost(ctx *context.Context) {
	editWebhook(ctx, giteaHookParams(ctx))
}

func giteaHookParams(ctx *context.Context) webhookParams {
	form := web.GetForm(ctx).(*forms.NewWebhookForm)

	contentType := webhook.ContentTypeJSON
	if webhook.HookContentType(form.ContentType) == webhook.ContentTypeForm {
		contentType = webhook.ContentTypeForm
	}

	return webhookParams{
		Type:        webhook_module.GITEA,
		URL:         form.PayloadURL,
		ContentType: contentType,
		Secret:      form.Secret,
		HTTPMethod:  form.HTTPMethod,
		WebhookForm: form.WebhookForm,
	}
}

// GogsHooksNewPost response for creating Gogs webhook
func GogsHooksNewPost(ctx *context.Context) {
	createWebhook(ctx, gogsHookParams(ctx))
}

// GogsHooksEditPost response for editing Gogs webhook
func GogsHooksEditPost(ctx *context.Context) {
	editWebhook(ctx, gogsHookParams(ctx))
}

func gogsHookParams(ctx *context.Context) webhookParams {
	form := web.GetForm(ctx).(*forms.NewGogshookForm)

	contentType := webhook.ContentTypeJSON
	if webhook.HookContentType(form.ContentType) == webhook.ContentTypeForm {
		contentType = webhook.ContentTypeForm
	}

	return webhookParams{
		Type:        webhook_module.GOGS,
		URL:         form.PayloadURL,
		ContentType: contentType,
		Secret:      form.Secret,
		WebhookForm: form.WebhookForm,
	}
}

// DiscordHooksNewPost response for creating Discord webhook
func DiscordHooksNewPost(ctx *context.Context) {
	createWebhook(ctx, discordHookParams(ctx))
}

// DiscordHooksEditPost response for editing Discord webhook
func DiscordHooksEditPost(ctx *context.Context) {
	editWebhook(ctx, discordHookParams(ctx))
}

func discordHookParams(ctx *context.Context) webhookParams {
	form := web.GetForm(ctx).(*forms.NewDiscordHookForm)

	return webhookParams{
		Type:        webhook_module.DISCORD,
		URL:         form.PayloadURL,
		ContentType: webhook.ContentTypeJSON,
		WebhookForm: form.WebhookForm,
		Meta: &webhook_service.DiscordMeta{
			Username: form.Username,
			IconURL:  form.IconURL,
		},
	}
}

// DingtalkHooksNewPost response for creating Dingtalk webhook
func DingtalkHooksNewPost(ctx *context.Context) {
	createWebhook(ctx, dingtalkHookParams(ctx))
}

// DingtalkHooksEditPost response for editing Dingtalk webhook
func DingtalkHooksEditPost(ctx *context.Context) {
	editWebhook(ctx, dingtalkHookParams(ctx))
}

func dingtalkHookParams(ctx *context.Context) webhookParams {
	form := web.GetForm(ctx).(*forms.NewDingtalkHookForm)

	return webhookParams{
		Type:        webhook_module.DINGTALK,
		URL:         form.PayloadURL,
		ContentType: webhook.ContentTypeJSON,
		WebhookForm: form.WebhookForm,
	}
}

// TelegramHooksNewPost response for creating Telegram webhook
func TelegramHooksNewPost(ctx *context.Context) {
	createWebhook(ctx, telegramHookParams(ctx))
}

// TelegramHooksEditPost response for editing Telegram webhook
func TelegramHooksEditPost(ctx *context.Context) {
	editWebhook(ctx, telegramHookParams(ctx))
}

func telegramHookParams(ctx *context.Context) webhookParams {
	form := web.GetForm(ctx).(*forms.NewTelegramHookForm)

	return webhookParams{
		Type:        webhook_module.TELEGRAM,
		URL:         fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=%s", url.PathEscape(form.BotToken), url.QueryEscape(form.ChatID)),
		ContentType: webhook.ContentTypeJSON,
		WebhookForm: form.WebhookForm,
		Meta: &webhook_service.TelegramMeta{
			BotToken: form.BotToken,
			ChatID:   form.ChatID,
		},
	}
}

// MatrixHooksNewPost response for creating Matrix webhook
func MatrixHooksNewPost(ctx *context.Context) {
	createWebhook(ctx, matrixHookParams(ctx))
}

// MatrixHooksEditPost response for editing Matrix webhook
func MatrixHooksEditPost(ctx *context.Context) {
	editWebhook(ctx, matrixHookParams(ctx))
}

func matrixHookParams(ctx *context.Context) webhookParams {
	form := web.GetForm(ctx).(*forms.NewMatrixHookForm)

	return webhookParams{
		Type:        webhook_module.MATRIX,
		URL:         fmt.Sprintf("%s/_matrix/client/r0/rooms/%s/send/m.room.message", form.HomeserverURL, url.PathEscape(form.RoomID)),
		ContentType: webhook.ContentTypeJSON,
		HTTPMethod:  http.MethodPut,
		WebhookForm: form.WebhookForm,
		Meta: &webhook_service.MatrixMeta{
			HomeserverURL: form.HomeserverURL,
			Room:          form.RoomID,
			MessageType:   form.MessageType,
		},
	}
}

// MSTeamsHooksNewPost response for creating MSTeams webhook
func MSTeamsHooksNewPost(ctx *context.Context) {
	createWebhook(ctx, mSTeamsHookParams(ctx))
}

// MSTeamsHooksEditPost response for editing MSTeams webhook
func MSTeamsHooksEditPost(ctx *context.Context) {
	editWebhook(ctx, mSTeamsHookParams(ctx))
}

func mSTeamsHookParams(ctx *context.Context) webhookParams {
	form := web.GetForm(ctx).(*forms.NewMSTeamsHookForm)

	return webhookParams{
		Type:        webhook_module.MSTEAMS,
		URL:         form.PayloadURL,
		ContentType: webhook.ContentTypeJSON,
		WebhookForm: form.WebhookForm,
	}
}

// SlackHooksNewPost response for creating Slack webhook
func SlackHooksNewPost(ctx *context.Context) {
	createWebhook(ctx, slackHookParams(ctx))
}

// SlackHooksEditPost response for editing Slack webhook
func SlackHooksEditPost(ctx *context.Context) {
	editWebhook(ctx, slackHookParams(ctx))
}

func slackHookParams(ctx *context.Context) webhookParams {
	form := web.GetForm(ctx).(*forms.NewSlackHookForm)

	return webhookParams{
		Type:        webhook_module.SLACK,
		URL:         form.PayloadURL,
		ContentType: webhook.ContentTypeJSON,
		WebhookForm: form.WebhookForm,
		Meta: &webhook_service.SlackMeta{
			Channel:  strings.TrimSpace(form.Channel),
			Username: form.Username,
			IconURL:  form.IconURL,
			Color:    form.Color,
		},
	}
}

// FeishuHooksNewPost response for creating Feishu webhook
func FeishuHooksNewPost(ctx *context.Context) {
	createWebhook(ctx, feishuHookParams(ctx))
}

// FeishuHooksEditPost response for editing Feishu webhook
func FeishuHooksEditPost(ctx *context.Context) {
	editWebhook(ctx, feishuHookParams(ctx))
}

func feishuHookParams(ctx *context.Context) webhookParams {
	form := web.GetForm(ctx).(*forms.NewFeishuHookForm)

	return webhookParams{
		Type:        webhook_module.FEISHU,
		URL:         form.PayloadURL,
		ContentType: webhook.ContentTypeJSON,
		WebhookForm: form.WebhookForm,
	}
}

// WechatworkHooksNewPost response for creating Wechatwork webhook
func WechatworkHooksNewPost(ctx *context.Context) {
	createWebhook(ctx, wechatworkHookParams(ctx))
}

// WechatworkHooksEditPost response for editing Wechatwork webhook
func WechatworkHooksEditPost(ctx *context.Context) {
	editWebhook(ctx, wechatworkHookParams(ctx))
}

func wechatworkHookParams(ctx *context.Context) webhookParams {
	form := web.GetForm(ctx).(*forms.NewWechatWorkHookForm)

	return webhookParams{
		Type:        webhook_module.WECHATWORK,
		URL:         form.PayloadURL,
		ContentType: webhook.ContentTypeJSON,
		WebhookForm: form.WebhookForm,
	}
}

// PackagistHooksNewPost response for creating Packagist webhook
func PackagistHooksNewPost(ctx *context.Context) {
	createWebhook(ctx, packagistHookParams(ctx))
}

// PackagistHooksEditPost response for editing Packagist webhook
func PackagistHooksEditPost(ctx *context.Context) {
	editWebhook(ctx, packagistHookParams(ctx))
}

func packagistHookParams(ctx *context.Context) webhookParams {
	form := web.GetForm(ctx).(*forms.NewPackagistHookForm)

	return webhookParams{
		Type:        webhook_module.PACKAGIST,
		URL:         fmt.Sprintf("https://packagist.org/api/update-package?username=%s&apiToken=%s", url.QueryEscape(form.Username), url.QueryEscape(form.APIToken)),
		ContentType: webhook.ContentTypeJSON,
		WebhookForm: form.WebhookForm,
		Meta: &webhook_service.PackagistMeta{
			Username:   form.Username,
			APIToken:   form.APIToken,
			PackageURL: form.PackageURL,
		},
	}
}

func checkWebhook(ctx *context.Context) (*ownerRepoCtx, *webhook.Webhook) {
	orCtx, err := getOwnerRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOwnerRepoCtx", err)
		return nil, nil
	}
	ctx.Data["BaseLink"] = orCtx.Link

	var w *webhook.Webhook
	if orCtx.RepoID > 0 {
		w, err = webhook.GetWebhookByRepoID(orCtx.RepoID, ctx.ParamsInt64(":id"))
	} else if orCtx.OwnerID > 0 {
		w, err = webhook.GetWebhookByOwnerID(orCtx.OwnerID, ctx.ParamsInt64(":id"))
	} else if orCtx.IsAdmin {
		w, err = webhook.GetSystemOrDefaultWebhook(ctx, ctx.ParamsInt64(":id"))
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
	case webhook_module.SLACK:
		ctx.Data["SlackHook"] = webhook_service.GetSlackHook(w)
	case webhook_module.DISCORD:
		ctx.Data["DiscordHook"] = webhook_service.GetDiscordHook(w)
	case webhook_module.TELEGRAM:
		ctx.Data["TelegramHook"] = webhook_service.GetTelegramHook(w)
	case webhook_module.MATRIX:
		ctx.Data["MatrixHook"] = webhook_service.GetMatrixHook(w)
	case webhook_module.PACKAGIST:
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

// TestWebhook test if web hook is work fine
func TestWebhook(ctx *context.Context) {
	hookID := ctx.ParamsInt64(":id")
	w, err := webhook.GetWebhookByRepoID(ctx.Repo.Repository.ID, hookID)
	if err != nil {
		ctx.Flash.Error("GetWebhookByRepoID: " + err.Error())
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

	apiUser := convert.ToUserWithAccessMode(ctx, ctx.Doer, perm.AccessModeNone)

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

	commitID := commit.ID.String()
	p := &api.PushPayload{
		Ref:          git.BranchPrefix + ctx.Repo.Repository.DefaultBranch,
		Before:       commitID,
		After:        commitID,
		CompareURL:   setting.AppURL + ctx.Repo.Repository.ComposeCompareURL(commitID, commitID),
		Commits:      []*api.PayloadCommit{apiCommit},
		TotalCommits: 1,
		HeadCommit:   apiCommit,
		Repo:         convert.ToRepo(ctx, ctx.Repo.Repository, perm.AccessModeNone),
		Pusher:       apiUser,
		Sender:       apiUser,
	}
	if err := webhook_service.PrepareWebhook(ctx, w, webhook_module.HookEventPush, p); err != nil {
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

	if err := webhook_service.ReplayHookTask(ctx, w, hookTaskUUID); err != nil {
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
