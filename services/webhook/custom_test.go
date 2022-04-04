// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package webhook

package webhook

import (
	"testing"

	webhook_model "code.gitea.io/gitea/models/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func TestGetCustomPayload(t *testing.T) {
	t.Run("Payload isn't altered.", func(t *testing.T) {
		p := createTestPayload()

		pl, err := GetCustomPayload(p, webhook_model.HookEventPush, "")
		require.NoError(t, err)
		require.Equal(t, p, pl)
	})
}

func TestWebhook_GetCustomHook(t *testing.T) {
	// Run with bearer token
	t.Run("GetCustomHook", func(t *testing.T) {
		w := &webhook_model.Webhook{
			Meta: `{"host_url": "http://localhost.com", "auth_token": "testToken"}`,
		}

		customHook := GetCustomHook(w)
		assert.Equal(t, *customHook, CustomMeta{
			HostURL:   "http://localhost.com",
			AuthToken: "testToken",
		})
	})
	// Run without bearer token
	t.Run("GetCustomHook", func(t *testing.T) {
		w := &webhook_model.Webhook{
			Meta: `{"host_url": "http://localhost.com"}`,
		}

		customHook := GetCustomHook(w)
		assert.Equal(t, *customHook, CustomMeta{
			HostURL: "http://localhost.com",
		})
	})
}
