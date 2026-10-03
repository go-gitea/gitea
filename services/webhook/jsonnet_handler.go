// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/json"
	"gitea.dev/modules/svg"
	webhook_module "gitea.dev/modules/webhook"
)

// JsonnetHandler delivers webhooks by evaluating jsonnet from a HookTypeDef.
type JsonnetHandler struct {
	Def *webhook_model.HookTypeDef
}

func (h JsonnetHandler) Type() webhook_module.HookType { return h.Def.Name }
func (h JsonnetHandler) DisplayName() string {
	if h.Def.DisplayName != "" {
		return h.Def.DisplayName
	}
	return h.Def.Name
}

func (h JsonnetHandler) DocsURL() string { return h.Def.DocsURL }

func (h JsonnetHandler) Icon(size int) template.HTML {
	if len(h.Def.IconData) > 0 {
		mime := h.Def.IconMIME
		if mime == "" {
			mime = "image/png"
		}
		b64 := base64.StdEncoding.EncodeToString(h.Def.IconData)
		return template.HTML(fmt.Sprintf(`<img alt width="%d" height="%d" src="data:%s;base64,%s">`, size, size, template.HTMLEscapeString(mime), b64)) //nolint:gosec
	}
	if h.Def.IconAsset != "" {
		if strings.HasPrefix(h.Def.IconAsset, "svg:") {
			return svg.RenderHTML(strings.TrimPrefix(h.Def.IconAsset, "svg:"), size, "img")
		}
		return imgIcon(h.Def.IconAsset, size)
	}
	return imgIcon("gogs.png", size)
}

func (h JsonnetHandler) FormFields() []FormField {
	if h.Def.FormSchema == "" {
		return nil
	}
	var fields []FormField
	if err := json.Unmarshal([]byte(h.Def.FormSchema), &fields); err != nil {
		return nil
	}
	return fields
}

func (h JsonnetHandler) Metadata(w *webhook_model.Webhook) any {
	if w.Meta == "" {
		return map[string]any{}
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(w.Meta), &meta); err != nil {
		return map[string]any{}
	}
	return meta
}

func (h JsonnetHandler) RequiresPayloadURL() bool {
	return h.Def.RequiresPayloadURL
}

func (h JsonnetHandler) UseAuthorizationHeader() string { return h.Def.UseAuthorizationHeader }
func (h JsonnetHandler) UseRequestSecret() string       { return h.Def.UseRequestSecret }

func (h JsonnetHandler) NewRequest(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	var event any
	if err := json.Unmarshal([]byte(t.PayloadContent), &event); err != nil {
		return nil, nil, fmt.Errorf("JsonnetHandler.NewRequest unmarshal event: %w", err)
	}
	meta := h.Metadata(w)
	ext := map[string]any{
		"event":      event,
		"event_type": string(t.EventType),
		"meta":       meta,
		"hook_type":  h.Def.Name,
		"webhook": map[string]any{
			"url":          w.URL,
			"secret":       w.Secret,
			"http_method":  w.HTTPMethod,
			"content_type": int(w.ContentType),
		},
	}

	if h.Def.RequestJsonnet != "" {
		return h.requestFromJsonnet(ctx, w, t, ext)
	}
	if h.Def.PayloadJsonnet != "" {
		return h.payloadFromJsonnet(ctx, w, t, ext)
	}
	// Fall back to legacy Go convertor when seeded without jsonnet yet.
	if r := GetLegacyRequester(h.Def.Name); r != nil {
		return r(ctx, w, t)
	}
	return nil, nil, fmt.Errorf("hook type %s has no jsonnet or legacy convertor", h.Def.Name)
}

func (h JsonnetHandler) requestFromJsonnet(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask, ext map[string]any) (*http.Request, []byte, error) {
	out, err := evaluateJsonnet(h.Def.RequestJsonnet, h.Def.Name+"/request.jsonnet", ext)
	if err != nil {
		return nil, nil, err
	}
	var result jsonnetRequestResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, nil, fmt.Errorf("parse request jsonnet result: %w", err)
	}
	method := result.Method
	if method == "" {
		method = w.HTTPMethod
	}
	if method == "" {
		method = http.MethodPost
	}
	urlStr := result.URL
	if urlStr == "" {
		urlStr = w.URL
	}
	body := []byte(result.Body)
	if len(body) == 0 {
		body = []byte("null")
	}
	// If body was a JSON value (object/array/string), keep as-is; json.RawMessage already raw.
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	for k, v := range result.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	withDefaults := true
	if result.WithDefaultHeaders != nil {
		withDefaults = *result.WithDefaultHeaders
	}
	if withDefaults {
		if err := addDefaultHeaders(req, []byte(w.Secret), w, t, body); err != nil {
			return nil, nil, err
		}
	}
	return req, body, nil
}

func (h JsonnetHandler) payloadFromJsonnet(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask, ext map[string]any) (*http.Request, []byte, error) {
	out, err := evaluateJsonnet(h.Def.PayloadJsonnet, h.Def.Name+"/payload.jsonnet", ext)
	if err != nil {
		return nil, nil, err
	}
	body := []byte(strings.TrimSpace(out))
	method := w.HTTPMethod
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, w.URL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := addDefaultHeaders(req, []byte(w.Secret), w, t, body); err != nil {
		return nil, nil, err
	}
	return req, body, nil
}
