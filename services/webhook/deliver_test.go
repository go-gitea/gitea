// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/hostmatcher"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookProxy(t *testing.T) {
	oldWebhook := setting.Webhook
	t.Cleanup(func() {
		setting.Webhook = oldWebhook
	})

	setting.Webhook.ProxyURL = "http://localhost:8080"
	setting.Webhook.ProxyURLFixed, _ = url.Parse(setting.Webhook.ProxyURL)
	setting.Webhook.ProxyHosts = []string{"*.discordapp.com", "discordapp.com"}

	allowedHostMatcher := hostmatcher.ParseHostMatchList("webhook.ALLOWED_HOST_LIST", "discordapp.com,s.discordapp.com")

	tests := []struct {
		req     string
		want    string
		wantErr bool
	}{
		{
			req:     "https://discordapp.com/api/webhooks/xxxxxxxxx/xxxxxxxxxxxxxxxxxxx",
			want:    "http://localhost:8080",
			wantErr: false,
		},
		{
			req:     "http://s.discordapp.com/assets/xxxxxx",
			want:    "http://localhost:8080",
			wantErr: false,
		},
		{
			req:     "http://github.com/a/b",
			want:    "",
			wantErr: false,
		},
		{
			req:     "http://www.discordapp.com/assets/xxxxxx",
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.req, func(t *testing.T) {
			req, err := http.NewRequest("POST", tt.req, nil)
			require.NoError(t, err)

			u, err := webhookProxy(allowedHostMatcher)(req)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			got := ""
			if u != nil {
				got = u.String()
			}
			assert.Equal(t, tt.want, got)
		})
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
		Type:        webhook_module.GITEA,
	}
	err := hook.SetHeaderAuthorization("Bearer s3cr3t-t0ken")
	assert.NoError(t, err)
	assert.NoError(t, webhook_model.CreateWebhook(db.DefaultContext, hook))

	hookTask := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadVersion: 2,
	}

	hookTask, err = webhook_model.CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	assert.NotNil(t, hookTask)

	assert.NoError(t, Deliver(t.Context(), hookTask))
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("waited to long for request to happen")
	}

	assert.True(t, hookTask.IsSucceed)
	assert.Equal(t, "******", hookTask.RequestInfo.Headers["Authorization"])
}

func TestWebhookDeliverHookTask(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	done := make(chan struct{}, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		switch r.URL.Path {
		case "/webhook/66d222a5d6349e1311f551e50722d837e30fce98":
			// Version 1
			assert.Equal(t, "push", r.Header.Get("X-GitHub-Event"))
			assert.Equal(t, "", r.Header.Get("Content-Type"))
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.Equal(t, `{"data": 42}`, string(body))

		case "/webhook/6db5dc1e282529a8c162c7fe93dd2667494eeb51":
			// Version 2
			assert.Equal(t, "push", r.Header.Get("X-GitHub-Event"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.Len(t, body, 2147)

		default:
			w.WriteHeader(404)
			t.Fatalf("unexpected url path %s", r.URL.Path)
			return
		}
		w.WriteHeader(200)
		done <- struct{}{}
	}))
	t.Cleanup(s.Close)

	hook := &webhook_model.Webhook{
		RepoID:      3,
		IsActive:    true,
		Type:        webhook_module.MATRIX,
		URL:         s.URL + "/webhook",
		HTTPMethod:  "PUT",
		ContentType: webhook_model.ContentTypeJSON,
		Meta:        `{"message_type":0}`, // text
	}
	assert.NoError(t, webhook_model.CreateWebhook(db.DefaultContext, hook))

	t.Run("Version 1", func(t *testing.T) {
		hookTask := &webhook_model.HookTask{
			HookID:         hook.ID,
			EventType:      webhook_module.HookEventPush,
			PayloadContent: `{"data": 42}`,
			PayloadVersion: 1,
		}

		hookTask, err := webhook_model.CreateHookTask(db.DefaultContext, hookTask)
		assert.NoError(t, err)
		assert.NotNil(t, hookTask)

		assert.NoError(t, Deliver(t.Context(), hookTask))
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("waited to long for request to happen")
		}

		assert.True(t, hookTask.IsSucceed)
	})

	t.Run("Version 2", func(t *testing.T) {
		p := pushTestPayload()
		data, err := p.JSONPayload()
		assert.NoError(t, err)

		hookTask := &webhook_model.HookTask{
			HookID:         hook.ID,
			EventType:      webhook_module.HookEventPush,
			PayloadContent: string(data),
			PayloadVersion: 2,
		}

		hookTask, err = webhook_model.CreateHookTask(db.DefaultContext, hookTask)
		assert.NoError(t, err)
		assert.NotNil(t, hookTask)

		assert.NoError(t, Deliver(t.Context(), hookTask))
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("waited to long for request to happen")
		}

		assert.True(t, hookTask.IsSucceed)
	})
}

func TestWebhookDeliverSpecificTypes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	type hookCase struct {
		gotBody    chan []byte
		httpMethod string // default to POST
	}

	cases := map[string]*hookCase{
		webhook_module.SLACK:      {},
		webhook_module.DISCORD:    {},
		webhook_module.DINGTALK:   {},
		webhook_module.TELEGRAM:   {},
		webhook_module.MSTEAMS:    {},
		webhook_module.FEISHU:     {},
		webhook_module.MATRIX:     {httpMethod: "PUT"},
		webhook_module.WECHATWORK: {},
		webhook_module.PACKAGIST:  {},
	}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		typ := strings.Split(r.URL.Path, "/")[1] // URL: "/{webhook_type}/other-path"
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"), r.URL.Path)
		assert.Equal(t, util.IfZero(cases[typ].httpMethod, "POST"), r.Method, "webhook test request %q", r.URL.Path)
		body, _ := io.ReadAll(r.Body) // read request and send it back to the test by testcase's chan
		cases[typ].gotBody <- body
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(s.Close)

	p := pushTestPayload()
	data, err := p.JSONPayload()
	assert.NoError(t, err)

	for typ := range cases {
		cases[typ].gotBody = make(chan []byte, 1)
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			hook := &webhook_model.Webhook{
				RepoID:   3,
				IsActive: true,
				Type:     typ,
				URL:      s.URL + "/" + typ,
				Meta:     "{}",
			}
			assert.NoError(t, webhook_model.CreateWebhook(db.DefaultContext, hook))

			hookTask := &webhook_model.HookTask{
				HookID:         hook.ID,
				EventType:      webhook_module.HookEventPush,
				PayloadContent: string(data),
				PayloadVersion: 2,
			}

			hookTask, err := webhook_model.CreateHookTask(db.DefaultContext, hookTask)
			assert.NoError(t, err)
			assert.NotNil(t, hookTask)

			assert.NoError(t, Deliver(t.Context(), hookTask))

			select {
			case gotBody := <-cases[typ].gotBody:
				assert.NotEqual(t, string(data), string(gotBody), "request body must be different from the event payload")
				assert.Equal(t, hookTask.RequestInfo.Body, string(gotBody), "delivered webhook payload doesn't match saved request")
			case <-time.After(5 * time.Second):
				t.Fatal("waited to long for request to happen")
			}

			assert.True(t, hookTask.IsSucceed)
		})
	}
}
