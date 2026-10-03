// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"fmt"
	"net/url"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/json"
	webhook_module "gitea.dev/modules/webhook"
)

func urlQueryEscape(s string) string { return url.QueryEscape(s) }

func legacyWebhookTask(params []any) (*webhook_model.Webhook, *webhook_model.HookTask, error) {
	hookType, _ := params[0].(string)
	eventType, _ := params[1].(string)
	event := params[2]
	meta := params[3]
	webhookURL, _ := params[4].(string)
	secret, _ := params[5].(string)
	httpMethod, _ := params[6].(string)
	contentTypeFloat, _ := params[7].(float64)

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal event: %w", err)
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal meta: %w", err)
	}
	if string(metaJSON) == "null" {
		metaJSON = []byte("{}")
	}

	w := &webhook_model.Webhook{
		URL:         webhookURL,
		HTTPMethod:  httpMethod,
		ContentType: webhook_model.HookContentType(contentTypeFloat),
		Secret:      secret,
		Type:        hookType,
		Meta:        string(metaJSON),
	}
	t := &webhook_model.HookTask{
		PayloadContent: string(eventJSON),
		EventType:      webhook_module.HookEventType(eventType),
		PayloadVersion: 2,
	}
	return w, t, nil
}

func legacyPayloadNative(params []any) (any, error) {
	hookType, _ := params[0].(string)
	requester := GetLegacyRequester(hookType)
	if requester == nil {
		return nil, fmt.Errorf("no legacy convertor for %s", hookType)
	}
	w, t, err := legacyWebhookTask(params)
	if err != nil {
		return nil, err
	}
	_, body, err := requester(context.Background(), w, t)
	if err != nil {
		return nil, err
	}
	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return string(body), nil
	}
	return parsed, nil
}

func legacyRequestNative(params []any) (any, error) {
	hookType, _ := params[0].(string)
	requester := GetLegacyRequester(hookType)
	if requester == nil {
		return nil, fmt.Errorf("no legacy convertor for %s", hookType)
	}
	w, t, err := legacyWebhookTask(params)
	if err != nil {
		return nil, err
	}
	req, body, err := requester(context.Background(), w, t)
	if err != nil {
		return nil, err
	}
	headers := map[string]any{}
	for k, vals := range req.Header {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}
	var parsedBody any
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		parsedBody = string(body)
	}
	return map[string]any{
		"method":               req.Method,
		"url":                  req.URL.String(),
		"headers":              headers,
		"body":                 parsedBody,
		"with_default_headers": false, // legacy requester already applied headers
	}, nil
}
