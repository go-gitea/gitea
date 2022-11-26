// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestWebhookProxy(t *testing.T) {
	setting.Webhook.ProxyURL = "http://localhost:8080"
	setting.Webhook.ProxyURLFixed, _ = url.Parse(setting.Webhook.ProxyURL)
	setting.Webhook.ProxyHosts = []string{"*.discordapp.com", "discordapp.com"}

	kases := map[string]string{
		"https://discordapp.com/api/webhooks/xxxxxxxxx/xxxxxxxxxxxxxxxxxxx": "http://localhost:8080",
		"http://s.discordapp.com/assets/xxxxxx":                             "http://localhost:8080",
		"http://github.com/a/b":                                             "",
	}

	for reqURL, proxyURL := range kases {
		req, err := http.NewRequest("POST", reqURL, nil)
		assert.NoError(t, err)

		u, err := webhookProxy()(req)
		assert.NoError(t, err)
		if proxyURL == "" {
			assert.Nil(t, u)
		} else {
			assert.EqualValues(t, proxyURL, u.String())
		}
	}
}

func TestWebhookDeliverAuthorizationHeader(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	done := make(chan struct{}, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/webhook", r.URL.Path)
		assert.Equal(t, "Bearer s3cr3t-t0ken", r.Header.Get("Authorization"))
		w.WriteHeader(200)
		done <- struct{}{}
	}))
	t.Cleanup(s.Close)

	hook := &webhook_model.Webhook{
		RepoID:      3,
		URL:         s.URL + "/webhook",
		ContentType: webhook_model.ContentTypeJSON,
		IsActive:    true,
		Type:        webhook_model.GITEA,
	}
	err := hook.SetHeaderAuthorization("Bearer s3cr3t-t0ken")
	assert.NoError(t, err)
	assert.NoError(t, webhook_model.CreateWebhook(db.DefaultContext, hook))
	db.GetEngine(db.DefaultContext).NoAutoTime().DB().Logger.ShowSQL(true)

	hookTask := &webhook_model.HookTask{HookID: hook.ID, EventType: webhook_model.HookEventPush, Payloader: &api.PushPayload{}}

	hookTask, err = webhook_model.CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	if !assert.NotNil(t, hookTask) {
		return
	}

	assert.NoError(t, Deliver(context.Background(), hookTask))
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("waited to long for request to happen")
	}

	assert.True(t, hookTask.IsSucceed)
}
