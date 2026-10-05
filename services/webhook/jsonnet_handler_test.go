// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	webhook_model "gitea.dev/models/webhook"

	"github.com/stretchr/testify/assert"
)

func TestJsonnetHandlerRequiresPayloadURL(t *testing.T) {
	discord := JsonnetHandler{Def: &webhook_model.HookTypeDef{
		RequiresPayloadURL: true,
		FormSchema:         `[{"id":"username","type":"text"},{"id":"icon_url","type":"url"}]`,
	}}
	assert.True(t, discord.RequiresPayloadURL(), "discord must show payload URL")

	slack := JsonnetHandler{Def: &webhook_model.HookTypeDef{
		RequiresPayloadURL: true,
		FormSchema:         `[{"id":"channel","type":"text"},{"id":"username","type":"text"}]`,
	}}
	assert.True(t, slack.RequiresPayloadURL(), "slack must show payload URL")

	packagist := JsonnetHandler{Def: &webhook_model.HookTypeDef{
		RequiresPayloadURL: false,
		FormSchema:         `[{"id":"username","type":"text"},{"id":"api_token","type":"secret"},{"id":"package_url","type":"url"}]`,
	}}
	assert.False(t, packagist.RequiresPayloadURL())

	telegram := JsonnetHandler{Def: &webhook_model.HookTypeDef{
		RequiresPayloadURL: false,
		FormSchema:         `[{"id":"bot_token","type":"secret"},{"id":"chat_id","type":"text"}]`,
	}}
	assert.False(t, telegram.RequiresPayloadURL())
}

func TestSeededBuiltinRequiresPayloadURL(t *testing.T) {
	RegisterBuiltinHandlers()
	assert.NoError(t, LoadHandlersFromDB(t.Context()))

	for _, name := range []string{"slack", "discord", "dingtalk", "msteams", "feishu", "wechatwork"} {
		h := GetHandler(name)
		assert.NotNil(t, h, name)
		assert.True(t, h.RequiresPayloadURL(), "%s should require payload URL", name)
	}
	for _, name := range []string{"telegram", "matrix", "packagist"} {
		h := GetHandler(name)
		assert.NotNil(t, h, name)
		assert.False(t, h.RequiresPayloadURL(), "%s should not require payload URL", name)
	}
}

func TestDeactivatedBuiltinStaysUnregistered(t *testing.T) {
	assert.NoError(t, LoadHandlersFromDB(t.Context()))
	ht, err := webhook_model.GetHookTypeByName(t.Context(), "discord")
	assert.NoError(t, err)
	ht.IsActive = false
	assert.NoError(t, webhook_model.UpdateHookType(t.Context(), ht))

	RegisterBuiltinHandlers() // simulates Init restart
	assert.NoError(t, LoadHandlersFromDB(t.Context()))
	assert.Nil(t, GetHandler("discord"), "deactivated builtin must not be registered after Init")

	// restore for other tests
	ht.IsActive = true
	assert.NoError(t, webhook_model.UpdateHookType(t.Context(), ht))
	RegisterBuiltinHandlers()
	assert.NoError(t, LoadHandlersFromDB(t.Context()))
	assert.NotNil(t, GetHandler("discord"))
}
