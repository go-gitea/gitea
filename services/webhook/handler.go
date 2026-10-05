// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"slices"
	"sync"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/svg"
	webhook_module "gitea.dev/modules/webhook"
)

// FormFieldType is the type of a webhook configuration form field.
type FormFieldType string

const (
	FormFieldText   FormFieldType = "text"
	FormFieldNumber FormFieldType = "number"
	FormFieldBool   FormFieldType = "bool"
	FormFieldSecret FormFieldType = "secret"
	FormFieldURL    FormFieldType = "url"
)

// FormField describes a type-specific configuration field shown when creating/editing a webhook.
type FormField struct {
	ID       string        `json:"id"`
	Label    string        `json:"label"`
	Type     FormFieldType `json:"type"`
	Required bool          `json:"required,omitempty"`
	Default  string        `json:"default,omitempty"`
	Help     string        `json:"help,omitempty"`
}

// Handler is the extension point for a webhook type (native Go or jsonnet-backed).
type Handler interface {
	Type() webhook_module.HookType
	DisplayName() string
	DocsURL() string
	Icon(size int) template.HTML
	FormFields() []FormField
	Metadata(w *webhook_model.Webhook) any
	NewRequest(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error)
	// RequiresPayloadURL is true when the form should show a free-form payload URL field.
	RequiresPayloadURL() bool
	// UseAuthorizationHeader controls Authorization header UI: "", "optional", or "required".
	UseAuthorizationHeader() string
	// UseRequestSecret controls Secret field UI: "", "optional", or "required".
	UseRequestSecret() string
}

type handlerRegistry struct {
	mu        sync.RWMutex
	byType    map[webhook_module.HookType]Handler
	order     []webhook_module.HookType
	requester map[webhook_module.HookType]Requester // legacy convertors used by jsonnet natives / v1 fallback
}

var registry = &handlerRegistry{
	byType:    map[webhook_module.HookType]Handler{},
	requester: map[webhook_module.HookType]Requester{},
}

// RegisterWebhookRequester registers a legacy payload convertor (also used by jsonnet natives).
func RegisterWebhookRequester(hookType webhook_module.HookType, requester Requester) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.requester[hookType] = requester
}

// RegisterHandler registers or replaces a webhook Handler.
func RegisterHandler(h Handler) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registerHandlerLocked(h)
}

func registerHandlerLocked(h Handler) {
	t := h.Type()
	if _, exists := registry.byType[t]; !exists {
		registry.order = append(registry.order, t)
	}
	registry.byType[t] = h
}

// UnregisterHandler removes a handler (e.g. when a custom type is deleted).
func UnregisterHandler(hookType webhook_module.HookType) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	delete(registry.byType, hookType)
	registry.order = slices.DeleteFunc(registry.order, func(t webhook_module.HookType) bool { return t == hookType })
}

// GetHandler returns the handler for a webhook type, or nil.
func GetHandler(name webhook_module.HookType) Handler {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.byType[name]
}

// ListHandlers returns handlers in registration order.
func ListHandlers() []Handler {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	out := make([]Handler, 0, len(registry.order))
	for _, t := range registry.order {
		if h, ok := registry.byType[t]; ok {
			out = append(out, h)
		}
	}
	return out
}

// IsValidHookTaskType returns true if a webhook type is registered.
func IsValidHookTaskType(name string) bool {
	return GetHandler(name) != nil
}

// GetLegacyRequester returns a registered legacy requester (for jsonnet natives / tests).
func GetLegacyRequester(hookType webhook_module.HookType) Requester {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.requester[hookType]
}

// SyncWebhookTypesSetting updates setting.Webhook.Types from the registry.
func SyncWebhookTypesSetting() {
	handlers := ListHandlers()
	types := make([]string, 0, len(handlers))
	for _, h := range handlers {
		types = append(types, h.Type())
	}
	setting.Webhook.Types = types
}

// requesterHandler wraps a legacy Requester as a Handler with static metadata.
type requesterHandler struct {
	hookType      webhook_module.HookType
	displayName   string
	docsURL       string
	iconHTML      func(size int) template.HTML
	fields        []FormField
	metaFn        func(*webhook_model.Webhook) any
	requester     Requester
	requiresURL   bool
	authorization string
	requestSecret string
}

func (h requesterHandler) Type() webhook_module.HookType { return h.hookType }
func (h requesterHandler) DisplayName() string           { return h.displayName }
func (h requesterHandler) DocsURL() string               { return h.docsURL }
func (h requesterHandler) Icon(size int) template.HTML {
	if h.iconHTML != nil {
		return h.iconHTML(size)
	}
	return imgIcon(h.hookType+".png", size)
}
func (h requesterHandler) FormFields() []FormField { return h.fields }
func (h requesterHandler) Metadata(w *webhook_model.Webhook) any {
	if h.metaFn != nil {
		return h.metaFn(w)
	}
	return nil
}

func (h requesterHandler) NewRequest(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	if h.requester == nil {
		return nil, nil, fmt.Errorf("no requester for %s", h.hookType)
	}
	return h.requester(ctx, w, t)
}
func (h requesterHandler) RequiresPayloadURL() bool       { return h.requiresURL }
func (h requesterHandler) UseAuthorizationHeader() string { return h.authorization }
func (h requesterHandler) UseRequestSecret() string       { return h.requestSecret }

func imgIcon(filename string, size int) template.HTML {
	src := setting.StaticURLPrefix + "/assets/img/" + filename
	return template.HTML(fmt.Sprintf(`<img alt width="%d" height="%d" src="%s">`, size, size, template.HTMLEscapeString(src))) //nolint:gosec // size is int; src escaped
}

func legacyRequester(hookType webhook_module.HookType) Requester {
	return func(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
		r := GetLegacyRequester(hookType)
		if r == nil {
			return nil, nil, fmt.Errorf("no legacy requester for %s", hookType)
		}
		return r(ctx, w, t)
	}
}

// RegisterBuiltinHandlers registers Go-backed handlers for built-in integration types.
// Called from Init after package init() has registered legacy requesters.
func RegisterBuiltinHandlers() {
	RegisterHandler(requesterHandler{
		hookType: webhook_module.SLACK, displayName: "Slack", docsURL: "https://slack.com", requiresURL: true, authorization: "optional",
		fields: []FormField{
			{ID: "channel", Label: "Channel", Type: FormFieldText, Required: true},
			{ID: "username", Label: "Username", Type: FormFieldText},
			{ID: "icon_url", Label: "Icon URL", Type: FormFieldURL},
			{ID: "color", Label: "Color", Type: FormFieldText},
		},
		metaFn:    func(w *webhook_model.Webhook) any { return GetSlackHook(w) },
		iconHTML:  func(size int) template.HTML { return imgIcon("slack.png", size) },
		requester: legacyRequester(webhook_module.SLACK),
	})
	RegisterHandler(requesterHandler{
		hookType: webhook_module.DISCORD, displayName: "Discord", requiresURL: true, authorization: "optional",
		fields: []FormField{
			{ID: "username", Label: "Username", Type: FormFieldText, Default: "Gitea"},
			{ID: "icon_url", Label: "Icon URL", Type: FormFieldURL},
		},
		metaFn:    func(w *webhook_model.Webhook) any { return GetDiscordHook(w) },
		iconHTML:  func(size int) template.HTML { return imgIcon("discord.png", size) },
		requester: legacyRequester(webhook_module.DISCORD),
	})
	RegisterHandler(requesterHandler{
		hookType: webhook_module.DINGTALK, displayName: "DingTalk", requiresURL: true, authorization: "optional", requestSecret: "optional",
		iconHTML:  func(size int) template.HTML { return imgIcon("dingtalk.ico", size) },
		requester: legacyRequester(webhook_module.DINGTALK),
	})
	RegisterHandler(requesterHandler{
		hookType: webhook_module.TELEGRAM, displayName: "Telegram", requiresURL: false, authorization: "optional", requestSecret: "optional",
		fields: []FormField{
			{ID: "bot_token", Label: "Bot Token", Type: FormFieldSecret, Required: true},
			{ID: "chat_id", Label: "Chat ID", Type: FormFieldText, Required: true},
			{ID: "thread_id", Label: "Thread ID", Type: FormFieldText},
		},
		metaFn:    func(w *webhook_model.Webhook) any { return GetTelegramHook(w) },
		iconHTML:  func(size int) template.HTML { return imgIcon("telegram.png", size) },
		requester: legacyRequester(webhook_module.TELEGRAM),
	})
	RegisterHandler(requesterHandler{
		hookType: webhook_module.MSTEAMS, displayName: "Microsoft Teams", requiresURL: true, authorization: "optional",
		iconHTML:  func(size int) template.HTML { return imgIcon("msteams.png", size) },
		requester: legacyRequester(webhook_module.MSTEAMS),
	})
	RegisterHandler(requesterHandler{
		hookType: webhook_module.FEISHU, displayName: "Feishu / Larksuite", requiresURL: true, requestSecret: "optional",
		iconHTML:  func(size int) template.HTML { return svg.RenderHTML("gitea-feishu", size, "img") },
		requester: legacyRequester(webhook_module.FEISHU),
	})
	RegisterHandler(requesterHandler{
		hookType: webhook_module.MATRIX, displayName: "Matrix", requiresURL: false, authorization: "optional",
		fields: []FormField{
			{ID: "homeserver_url", Label: "Homeserver URL", Type: FormFieldURL, Required: true},
			{ID: "room_id", Label: "Room ID", Type: FormFieldText, Required: true},
			{ID: "message_type", Label: "Message Type", Type: FormFieldNumber, Default: "1"},
		},
		metaFn:    func(w *webhook_model.Webhook) any { return GetMatrixHook(w) },
		iconHTML:  func(size int) template.HTML { return svg.RenderHTML("gitea-matrix", size, "img") },
		requester: legacyRequester(webhook_module.MATRIX),
	})
	RegisterHandler(requesterHandler{
		hookType: webhook_module.WECHATWORK, displayName: "WeChat Work", requiresURL: true, authorization: "optional",
		iconHTML:  func(size int) template.HTML { return imgIcon("wechatwork.png", size) },
		requester: legacyRequester(webhook_module.WECHATWORK),
	})
	RegisterHandler(requesterHandler{
		hookType: webhook_module.PACKAGIST, displayName: "Packagist", requiresURL: false, authorization: "optional",
		fields: []FormField{
			{ID: "username", Label: "Username", Type: FormFieldText, Required: true},
			{ID: "api_token", Label: "API Token", Type: FormFieldSecret, Required: true},
			{ID: "package_url", Label: "Package URL", Type: FormFieldURL, Required: true},
		},
		metaFn:    func(w *webhook_model.Webhook) any { return GetPackagistHook(w) },
		iconHTML:  func(size int) template.HTML { return imgIcon("packagist.png", size) },
		requester: legacyRequester(webhook_module.PACKAGIST),
	})
	SyncWebhookTypesSetting()
	log.Debug("Registered %d webhook handlers", len(ListHandlers()))
}
