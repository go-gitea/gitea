// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/models/webhook"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/web"
	webhook_module "gitea.dev/modules/webhook"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	"gitea.dev/services/forms"
	webhook_service "gitea.dev/services/webhook"
)

const (
	tplHooks        templates.TplName = "repo/settings/webhook/base"
	tplHookNew      templates.TplName = "repo/settings/webhook/new"
	tplOrgHookNew   templates.TplName = "org/settings/hook_new"
	tplUserHookNew  templates.TplName = "user/settings/hook_new"
	tplAdminHookNew templates.TplName = "admin/hook_new"
)

// Webhooks render web hooks list page
func Webhooks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.hooks")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["BaseLink"] = ctx.Repo.RepoLink + "/settings/hooks"
	ctx.Data["BaseLinkNew"] = ctx.Repo.RepoLink + "/settings/hooks"
	ctx.Data["Description"] = ctx.Tr("repo.settings.hooks_desc", "https://docs.gitea.com/usage/webhooks")
	ctx.Data["WebhookHandlers"] = webhook_service.ListHandlers()

	ws, err := db.Find[webhook.Webhook](ctx, webhook.ListWebhookOptions{RepoID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.ServerError("GetWebhooksByRepoID", err)
		return
	}
	ctx.Data["Webhooks"] = ws

	ctx.HTML(http.StatusOK, tplHooks)
}

type ownerRepoCtx struct {
	Owner           *user_model.User
	Repo            *repo_model.Repository
	OwnerID         int64
	RepoID          int64
	IsAdmin         bool
	IsSystemWebhook bool
	Link            string
	LinkNew         string
	NewTemplate     templates.TplName
}

// getOwnerRepoCtx determines whether this is a repo, owner, or admin (both default and system) context.
func getOwnerRepoCtx(ctx *context.Context) (*ownerRepoCtx, error) {
	if ctx.Data["PageIsRepoSettings"] == true {
		return &ownerRepoCtx{
			Repo:        ctx.Repo.Repository,
			RepoID:      ctx.Repo.Repository.ID,
			Link:        path.Join(ctx.Repo.RepoLink, "settings/hooks"),
			LinkNew:     path.Join(ctx.Repo.RepoLink, "settings/hooks"),
			NewTemplate: tplHookNew,
		}, nil
	}

	if ctx.Data["PageIsOrgSettings"] == true {
		return &ownerRepoCtx{
			Owner:       ctx.ContextUser,
			OwnerID:     ctx.ContextUser.ID,
			Link:        path.Join(ctx.Org.OrgLink, "settings/hooks"),
			LinkNew:     path.Join(ctx.Org.OrgLink, "settings/hooks"),
			NewTemplate: tplOrgHookNew,
		}, nil
	}

	if ctx.Data["PageIsUserSettings"] == true {
		return &ownerRepoCtx{
			Owner:       ctx.Doer,
			OwnerID:     ctx.Doer.ID,
			Link:        path.Join(setting.AppSubURL, "/user/settings/hooks"),
			LinkNew:     path.Join(setting.AppSubURL, "/user/settings/hooks"),
			NewTemplate: tplUserHookNew,
		}, nil
	}

	if ctx.Data["PageIsAdmin"] == true {
		return &ownerRepoCtx{
			IsAdmin:         true,
			IsSystemWebhook: ctx.PathParam("configType") == "system-hooks",
			Link:            path.Join(setting.AppSubURL, "/-/admin/hooks"),
			LinkNew:         path.Join(setting.AppSubURL, "/-/admin/", ctx.PathParam("configType")),
			NewTemplate:     tplAdminHookNew,
		}, nil
	}

	return nil, errors.New("unable to set OwnerRepo context")
}

// recordWebhookAudit emits a webhook audit event scoped to the repository,
// organization, user, or instance (admin/system) the webhook belongs to. The
// shared add/edit handlers run in any of these contexts, so the scope is derived
// from orCtx rather than assuming a repository.
func (orCtx *ownerRepoCtx) recordWebhookAudit(ctx *context.Context, actions audit.ScopedActions, url string) {
	audit.RecordScoped(ctx, orCtx.Owner, orCtx.Repo, actions, "webhook", url)
}

func checkHookType(ctx *context.Context) string {
	hookType := strings.ToLower(ctx.PathParam("type"))
	if !webhook_service.IsValidHookTaskType(hookType) {
		ctx.NotFound(nil)
		return ""
	}
	return hookType
}

func setWebhookHandlerData(ctx *context.Context, hookType string, w *webhook.Webhook) {
	ctx.Data["WebhookHandlers"] = webhook_service.ListHandlers()
	h := webhook_service.GetHandler(hookType)
	ctx.Data["WebhookHandler"] = h
	meta := map[string]any{}
	if h != nil && w != nil {
		switch m := h.Metadata(w).(type) {
		case map[string]any:
			meta = m
		case nil:
			// leave empty
		default:
			// struct meta → map via JSON
			if b, err := json.Marshal(m); err == nil {
				_ = json.Unmarshal(b, &meta)
			}
		}
	}
	ctx.Data["HookMeta"] = meta
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
	ctx.Data["BaseLink"] = orCtx.LinkNew
	ctx.Data["BaseLinkNew"] = orCtx.LinkNew
	w := &webhook.Webhook{HookEvent: &webhook_module.HookEvent{}}
	setWebhookHandlerData(ctx, hookType, w)
	if hookType == "discord" {
		if meta, ok := ctx.Data["HookMeta"].(map[string]any); ok {
			if _, has := meta["username"]; !has {
				meta["username"] = "Gitea"
			}
		}
	}

	ctx.HTML(http.StatusOK, orCtx.NewTemplate)
}

// ParseHookEvent convert web form content to webhook.HookEvent
func ParseHookEvent(form forms.WebhookForm) *webhook_module.HookEvent {
	return &webhook_module.HookEvent{
		PushOnly:       form.PushOnly(),
		SendEverything: form.SendEverything(),
		ChooseEvents:   form.ChooseEvents(),
		HookEvents: webhook_module.HookEvents{
			webhook_module.HookEventCreate:                   form.Create,
			webhook_module.HookEventDelete:                   form.Delete,
			webhook_module.HookEventFork:                     form.Fork,
			webhook_module.HookEventIssues:                   form.Issues,
			webhook_module.HookEventIssueAssign:              form.IssueAssign,
			webhook_module.HookEventIssueLabel:               form.IssueLabel,
			webhook_module.HookEventIssueMilestone:           form.IssueMilestone,
			webhook_module.HookEventIssueComment:             form.IssueComment,
			webhook_module.HookEventRelease:                  form.Release,
			webhook_module.HookEventPush:                     form.Push,
			webhook_module.HookEventPullRequest:              form.PullRequest,
			webhook_module.HookEventPullRequestAssign:        form.PullRequestAssign,
			webhook_module.HookEventPullRequestLabel:         form.PullRequestLabel,
			webhook_module.HookEventPullRequestMilestone:     form.PullRequestMilestone,
			webhook_module.HookEventPullRequestComment:       form.PullRequestComment,
			webhook_module.HookEventPullRequestReview:        form.PullRequestReview,
			webhook_module.HookEventPullRequestSync:          form.PullRequestSync,
			webhook_module.HookEventPullRequestReviewRequest: form.PullRequestReviewRequest,
			webhook_module.HookEventWiki:                     form.Wiki,
			webhook_module.HookEventRepository:               form.Repository,
			webhook_module.HookEventPackage:                  form.Package,
			webhook_module.HookEventStatus:                   form.Status,
			webhook_module.HookEventWorkflowRun:              form.WorkflowRun,
			webhook_module.HookEventWorkflowJob:              form.WorkflowJob,
		},
		BranchFilter: form.BranchFilter,
	}
}

type webhookParams struct {
	// Type should be imported from webhook package (webhook.XXX)
	Type string

	URL         string
	ContentType webhook.HookContentType
	HTTPMethod  string
	WebhookForm forms.WebhookForm
	Meta        any
}

func createWebhook(ctx *context.Context, params webhookParams) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.add_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	wEmpty := webhook.Webhook{HookEvent: &webhook_module.HookEvent{}}
	ctx.Data["Webhook"] = wEmpty
	ctx.Data["HookType"] = params.Type

	orCtx, err := getOwnerRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOwnerRepoCtx", err)
		return
	}
	ctx.Data["BaseLink"] = orCtx.LinkNew
	ctx.Data["BaseLinkNew"] = orCtx.LinkNew
	setWebhookHandlerData(ctx, params.Type, &wEmpty)

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
		Name:            strings.TrimSpace(params.WebhookForm.Name),
		HTTPMethod:      params.HTTPMethod,
		ContentType:     params.ContentType,
		Secret:          params.WebhookForm.Secret,
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

	orCtx.recordWebhookAudit(ctx, audit.WebhookAdd, w.URL)

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
	setWebhookHandlerData(ctx, params.Type, w)

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
	w.Name = strings.TrimSpace(params.WebhookForm.Name)
	w.ContentType = params.ContentType
	w.Secret = params.WebhookForm.Secret
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
	} else if err := webhook.UpdateWebhook(ctx, w); err != nil {
		ctx.ServerError("UpdateWebhook", err)
		return
	}

	orCtx.recordWebhookAudit(ctx, audit.WebhookUpdate, w.URL)

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// HooksNewPost creates a webhook for any registered non-gitea/gogs type using form schema.
func HooksNewPost(ctx *context.Context) {
	createWebhook(ctx, genericHookParams(ctx))
}

// HooksEditPost edits a webhook for any registered non-gitea/gogs type using form schema.
func HooksEditPost(ctx *context.Context) {
	editWebhook(ctx, genericHookParams(ctx))
}

func genericHookParams(ctx *context.Context) webhookParams {
	form := web.GetForm[*forms.NewGenericHookForm](ctx)
	hookType := strings.ToLower(ctx.PathParam("type"))
	h := webhook_service.GetHandler(hookType)
	if h == nil {
		ctx.NotFound(nil)
		return webhookParams{}
	}

	meta := map[string]any{}
	valid := true
	for _, field := range h.FormFields() {
		key := "meta_" + field.ID
		switch field.Type {
		case webhook_service.FormFieldBool:
			meta[field.ID] = ctx.FormBool(key)
		case webhook_service.FormFieldNumber:
			meta[field.ID] = ctx.FormInt(key)
			if field.Required && ctx.FormString(key) == "" {
				ctx.Data["Err_"+field.ID] = true
				ctx.Flash.Error(ctx.Tr("form.required_field", field.Label))
				valid = false
			}
		default:
			meta[field.ID] = strings.TrimSpace(ctx.FormString(key))
			if field.Required {
				if v, ok := meta[field.ID].(string); ok && v == "" {
					ctx.Data["Err_"+field.ID] = true
					ctx.Flash.Error(ctx.Tr("form.required_field", field.Label))
					valid = false
				}
			}
		}
	}

	payloadURL := strings.TrimSpace(form.PayloadURL)
	httpMethod := http.MethodPost

	// Preserve URL construction for known builtins that embed credentials in the URL.
	switch hookType {
	case webhook_module.TELEGRAM:
		botToken, _ := meta["bot_token"].(string)
		chatID, _ := meta["chat_id"].(string)
		threadID, _ := meta["thread_id"].(string)
		payloadURL = fmt.Sprintf("https://api.telegram.org/bot%s/sendRichMessage?chat_id=%s&message_thread_id=%s", url.PathEscape(botToken), url.QueryEscape(chatID), url.QueryEscape(threadID))
	case webhook_module.MATRIX:
		homeserver, _ := meta["homeserver_url"].(string)
		roomID, _ := meta["room_id"].(string)
		payloadURL = fmt.Sprintf("%s/_matrix/client/r0/rooms/%s/send/m.room.message", homeserver, matrixRoomIDEncode(roomID))
		httpMethod = http.MethodPut
	case webhook_module.PACKAGIST:
		username, _ := meta["username"].(string)
		token, _ := meta["api_token"].(string)
		payloadURL = fmt.Sprintf("https://packagist.org/api/update-package?username=%s&apiToken=%s", url.QueryEscape(username), url.QueryEscape(token))
	}

	if h.RequiresPayloadURL() && payloadURL == "" {
		ctx.Flash.Error(ctx.Tr("form.required_field", "URL"))
		valid = false
	}
	if !valid {
		return webhookParams{Type: hookType, WebhookForm: form.WebhookForm, Meta: meta}
	}

	return webhookParams{
		Type:        hookType,
		URL:         payloadURL,
		ContentType: webhook.ContentTypeJSON,
		HTTPMethod:  httpMethod,
		WebhookForm: form.WebhookForm,
		Meta:        meta,
	}
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
	form := web.GetForm[*forms.NewWebhookForm](ctx)

	contentType := webhook.ContentTypeJSON
	if webhook.HookContentType(form.ContentType) == webhook.ContentTypeForm {
		contentType = webhook.ContentTypeForm
	}

	return webhookParams{
		Type:        webhook_module.GITEA,
		URL:         form.PayloadURL,
		ContentType: contentType,
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
	form := web.GetForm[*forms.NewGogshookForm](ctx)

	contentType := webhook.ContentTypeJSON
	if webhook.HookContentType(form.ContentType) == webhook.ContentTypeForm {
		contentType = webhook.ContentTypeForm
	}

	return webhookParams{
		Type:        webhook_module.GOGS,
		URL:         form.PayloadURL,
		ContentType: contentType,
		WebhookForm: form.WebhookForm,
	}
}

func matrixRoomIDEncode(roomID string) string {
	// See https://spec.matrix.org/latest/appendices/#room-ids
	return strings.NewReplacer("%21", "!", "%3A", ":").Replace(url.PathEscape(roomID))
}

func checkWebhook(ctx *context.Context) (*ownerRepoCtx, *webhook.Webhook) {
	orCtx, err := getOwnerRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOwnerRepoCtx", err)
		return nil, nil
	}
	ctx.Data["BaseLink"] = orCtx.Link
	ctx.Data["BaseLinkNew"] = orCtx.LinkNew

	var w *webhook.Webhook
	if orCtx.RepoID > 0 {
		w, err = webhook.GetWebhookByRepoID(ctx, orCtx.RepoID, ctx.PathParamInt64("id"))
	} else if orCtx.OwnerID > 0 {
		w, err = webhook.GetWebhookByOwnerID(ctx, orCtx.OwnerID, ctx.PathParamInt64("id"))
	} else if orCtx.IsAdmin {
		w, err = webhook.GetSystemOrDefaultWebhook(ctx, ctx.PathParamInt64("id"))
	}
	if err != nil || w == nil {
		if webhook.IsErrWebhookNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetWebhookByID", err)
		}
		return nil, nil
	}

	ctx.Data["HookType"] = w.Type
	setWebhookHandlerData(ctx, w.Type, w)

	ctx.Data["History"], err = w.History(ctx, 1)
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
	hookID := ctx.PathParamInt64("id")
	w, err := webhook.GetWebhookByRepoID(ctx, ctx.Repo.Repository.ID, hookID)
	if err != nil {
		ctx.Flash.Error("GetWebhookByRepoID: " + err.Error())
		ctx.Status(http.StatusInternalServerError)
		return
	}

	// use a fake commit to test webhook
	ghostUser := user_model.NewGhostUser()
	objectFormat := git.ObjectFormatFromName(ctx.Repo.Repository.ObjectFormatName)
	commit := &git.Commit{
		ID:            objectFormat.EmptyObjectID(),
		Author:        ghostUser.NewGitSig(),
		Committer:     ghostUser.NewGitSig(),
		CommitMessage: git.CommitMessage{MessageRaw: "This is a fake commit for webhook push test"},
	}

	apiUser := convert.ToUserWithAccessMode(ctx, ctx.Doer, perm.AccessModeNone)

	apiCommit := &api.PayloadCommit{
		ID:      commit.ID.String(),
		Message: commit.MessageUTF8(),
		URL:     ctx.Repo.Repository.HTMLURL(ctx) + "/commit/" + url.PathEscape(commit.ID.String()),
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
		Ref:          git.RefNameFromBranch(ctx.Repo.Repository.DefaultBranch).String(),
		Before:       commitID,
		After:        commitID,
		CompareURL:   setting.AppURL + ctx.Repo.Repository.ComposeCompareURL(commitID, commitID),
		Commits:      []*api.PayloadCommit{apiCommit},
		TotalCommits: 1,
		HeadCommit:   apiCommit,
		Repo:         convert.ToRepo(ctx, ctx.Repo.Repository, access_model.Permission{AccessMode: perm.AccessModeNone}),
		Pusher:       apiUser,
		Sender:       apiUser,
	}
	if err := webhook_service.PrepareTestWebhook(ctx, w, webhook_module.HookEventPush, p); err != nil {
		ctx.Flash.Error("PrepareTestWebhook: " + err.Error())
		ctx.Status(http.StatusInternalServerError)
	} else {
		ctx.Flash.Info(ctx.Tr("repo.settings.webhook.delivery.success"))
		ctx.Status(http.StatusOK)
	}
}

// ReplayWebhook replays a webhook
func ReplayWebhook(ctx *context.Context) {
	hookTaskUUID := ctx.PathParam("uuid")

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}

	if err := webhook_service.ReplayHookTask(ctx, w, hookTaskUUID); err != nil {
		if webhook.IsErrHookTaskNotExist(err) {
			ctx.NotFound(nil)
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
	hook, err := webhook.GetWebhookByRepoID(ctx, ctx.Repo.Repository.ID, ctx.FormInt64("id"))
	if err != nil {
		ctx.Flash.Error("GetWebhookByRepoID: " + err.Error())
	} else if err := webhook.DeleteWebhookByRepoID(ctx, ctx.Repo.Repository.ID, hook.ID); err != nil {
		ctx.Flash.Error("DeleteWebhookByRepoID: " + err.Error())
	} else {
		audit.RecordScoped(ctx, nil, ctx.Repo.Repository, audit.WebhookRemove, "webhook", hook.URL)

		ctx.Flash.Success(ctx.Tr("repo.settings.webhook_deletion_success"))
	}

	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/hooks")
}
