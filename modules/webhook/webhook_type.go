// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
)

// WebhookType represents a webhook type, all webhook types should implement this interface
type WebhookType interface {
	Name() string
	GetPayload(p api.Payloader, event models.HookEventType, meta string) (api.Payloader, error)
}

var (
	webhookTypes        map[string]WebhookType
	defaultWebhookTypes = []WebhookType{
		&SlackWebhookType{},
		&TelegramWebhookType{},
		&DingtalkWebhookType{},
		&DiscordWebhookType{},
		&MSTeamsWebhookType{},
	}
)

func init() {
	webhookTypes = make(map[string]WebhookType)
}

// RegisterWebhookType register a webhook type
func RegisterWebhookType(t WebhookType) {
	webhookTypes[t.Name()] = t
}
