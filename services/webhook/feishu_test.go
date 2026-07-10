// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	webhook_model "gitea.dev/models/webhook"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFeishuGetTenantAccessToken verifies that the tenant_access_token is read
// from the top level of the response (the Feishu token endpoint does not wrap
// it in a "data" object). A previous bug parsed r.Data.TenantAccessToken, which
// is always empty, so the access token was never attached and Feishu returned
// 99991661 "Missing access token for authorization".
func TestFeishuGetTenantAccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"success","tenant_access_token":"t-fake-token","expire":7200}`))
	}))
	defer srv.Close()

	origBase := feishuAPIBaseURL
	feishuAPIBaseURL = srv.URL
	defer func() { feishuAPIBaseURL = origBase }()

	token, expire, err := feishuGetTenantAccessToken(context.Background(), feishuAPIBaseURL, "app_id", "app_secret")
	require.NoError(t, err)
	assert.Equal(t, "t-fake-token", token)
	assert.Equal(t, 7200, expire)
}

// TestFeishuGetTenantAccessTokenEmpty ensures an error is returned when the
// token endpoint succeeds but returns an empty tenant_access_token, so the bug
// surfaces as a clear error instead of a missing Authorization header.
func TestFeishuGetTenantAccessTokenEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"success","tenant_access_token":"","expire":7200}`))
	}))
	defer srv.Close()

	origBase := feishuAPIBaseURL
	feishuAPIBaseURL = srv.URL
	defer func() { feishuAPIBaseURL = origBase }()

	_, _, err := feishuGetTenantAccessToken(context.Background(), feishuAPIBaseURL, "app_id", "app_secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty tenant_access_token")
}

// TestFeishuGetAccessTokenCaching verifies that the underlying token endpoint
// is only hit once within the cache window: subsequent calls reuse the cached
// tenant_access_token instead of requesting a new one every time.
func TestFeishuGetAccessTokenCaching(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"success","tenant_access_token":"t-cached","expire":7200}`))
	}))
	defer srv.Close()

	origBase := feishuAPIBaseURL
	feishuAPIBaseURL = srv.URL
	defer func() { feishuAPIBaseURL = origBase }()

	// Start from a clean cache so the call count is deterministic.
	feishuTokenMu.Lock()
	feishuTokenCache = map[string]feishuTokenEntry{}
	feishuTokenMu.Unlock()

	var calls int
	orig := feishuGetTenantAccessTokenFunc
	feishuGetTenantAccessTokenFunc = func(ctx context.Context, baseURL, appID, appSecret string) (string, int, error) {
		calls++
		return feishuGetTenantAccessToken(ctx, baseURL, appID, appSecret)
	}
	defer func() { feishuGetTenantAccessTokenFunc = orig }()

	for range 3 {
		tok, err := feishuGetAccessToken(context.Background(), feishuAPIBaseURL, "app_id", "app_secret")
		require.NoError(t, err)
		assert.Equal(t, "t-cached", tok)
	}
	assert.Equal(t, 1, calls, "underlying token endpoint should be called only once within the cache window")
}

func TestFeishuPayload(t *testing.T) {
	fc := feishuConvertor{}
	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := fc.Create(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] branch test created`, pl.Content.Text)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := fc.Delete(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] branch test deleted`, pl.Content.Text)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := fc.Fork(p)
		require.NoError(t, err)

		assert.Equal(t, `test/repo2 is forked to test/repo`, pl.Content.Text)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := fc.Push(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo:test] \r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", pl.Content.Text)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := fc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[Issue-test/repo #2]: opened\ncrash\nhttp://localhost:3000/test/repo/issues/2\nIssue by user1\nOperator: user1\nAssignees: user1\n\nissue body", pl.Content.Text)

		p.Action = api.HookIssueClosed
		pl, err = fc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[Issue-test/repo #2]: closed\ncrash\nhttp://localhost:3000/test/repo/issues/2\nIssue by user1\nOperator: user1\nAssignees: user1\n\nissue body", pl.Content.Text)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := fc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, "[Comment-test/repo #2]: created\ncrash\nhttp://localhost:3000/test/repo/issues/2\nIssue by user1\nOperator: user1\n\nmore info needed", pl.Content.Text)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := fc.PullRequest(p)
		require.NoError(t, err)

		assert.Equal(t, "[PullRequest-test/repo #12]: opened\nFix bug\nhttp://localhost:3000/test/repo/pulls/12\nPullRequest by user1\nOperator: user1\nAssignees: user1\n\nfixes bug #2", pl.Content.Text)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := fc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, "[Comment-test/repo #12]: created\nFix bug\nhttp://localhost:3000/test/repo/pulls/12\nPullRequest by user1\nOperator: user1\n\nchanges requested", pl.Content.Text)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := fc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Pull request review approved : #12 Fix bug\r\n\r\ngood job", pl.Content.Text)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := fc.Repository(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Repository created", pl.Content.Text)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := fc.Package(p)
		require.NoError(t, err)

		assert.Equal(t, "Package created: GiteaContainer:latest by user1", pl.Content.Text)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := fc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] New wiki page 'index' (Wiki change comment) by user1", pl.Content.Text)

		p.Action = api.HookWikiEdited
		pl, err = fc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Wiki page 'index' edited (Wiki change comment) by user1", pl.Content.Text)

		p.Action = api.HookWikiDeleted
		pl, err = fc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Wiki page 'index' deleted by user1", pl.Content.Text)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := fc.Release(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Release created: v1.0 by user1", pl.Content.Text)
	})
}

func TestFeishuJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		RepoID:     3,
		IsActive:   true,
		Type:       webhook_module.FEISHU,
		URL:        "https://feishu.example.com/",
		Meta:       `{"app_id":"app_id","app_secret":"app_secret"}`,
		HTTPMethod: "POST",
		Secret:     "secret",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := newFeishuRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	// A push event has no mentioned users, so no direct message is delivered.
	// The framework request validates the app credentials against the token
	// endpoint instead, which is what the user observes in the delivery log.
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal/", req.URL.String())
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.JSONEq(t, `{"app_id":"app_id","app_secret":"app_secret"}`, string(reqBody))
}

// TestFeishuJSONPayloadWithRecipients verifies that when an event has direct
// message recipients, the framework request itself is a real Feishu direct
// message (sent to the im/v1/messages endpoint with a bearer token) instead of
// only probing the token endpoint. This was the root cause of the previous bug:
// the actual direct messages were fired asynchronously via the SDK and silently
// dropped.
func TestFeishuJSONPayloadWithRecipients(t *testing.T) {
	// Avoid any real Feishu API call: substitute a mock token getter.
	orig := feishuGetAccessTokenFunc
	feishuGetAccessTokenFunc = func(ctx context.Context, baseURL, appID, appSecret string) (string, error) {
		return "fake-tenant-token", nil
	}
	defer func() { feishuGetAccessTokenFunc = orig }()

	p := issueTestPayload()
	p.Action = api.HookIssueOpened

	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		Type:       webhook_module.FEISHU,
		Meta:       `{"app_id":"app_id","app_secret":"app_secret"}`,
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		EventType:          webhook_module.HookEventIssues,
		PayloadContent:     string(data),
		PayloadVersion:     2,
		DeliveryRecipients: []string{"user@example.com"},
	}

	req, reqBody, err := newFeishuRequest(t.Context(), hook, task)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)

	// The framework request is a real direct message, not a token probe.
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, feishuAPIBaseURL+"/open-apis/im/v1/messages?receive_id_type=email", req.URL.String())
	assert.Equal(t, "Bearer fake-tenant-token", req.Header.Get("Authorization"))
	assert.Contains(t, string(reqBody), `"receive_id":"user@example.com"`)
	assert.Contains(t, string(reqBody), `"msg_type":"text"`)
}

// TestFeishuJSONPayloadWithMultipleRecipients verifies that when an event has
// several recipients, the first one is delivered by the framework request while
// the remaining ones are sent via direct Open API calls (no Feishu SDK
// involved).
func TestFeishuJSONPayloadWithMultipleRecipients(t *testing.T) {
	origToken := feishuGetAccessTokenFunc
	feishuGetAccessTokenFunc = func(ctx context.Context, baseURL, appID, appSecret string) (string, error) {
		return "fake-token", nil
	}
	defer func() { feishuGetAccessTokenFunc = origToken }()

	var sent []string
	origSend := feishuSendMessageFunc
	feishuSendMessageFunc = func(ctx context.Context, baseURL, token, receiveID, text string) error {
		sent = append(sent, receiveID)
		return nil
	}
	defer func() { feishuSendMessageFunc = origSend }()

	p := issueTestPayload()
	p.Action = api.HookIssueOpened

	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		Type:       webhook_module.FEISHU,
		Meta:       `{"app_id":"app_id","app_secret":"app_secret"}`,
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		EventType:          webhook_module.HookEventIssues,
		PayloadContent:     string(data),
		PayloadVersion:     2,
		DeliveryRecipients: []string{"first@example.com", "second@example.com", "third@example.com"},
	}

	req, reqBody, err := newFeishuRequest(t.Context(), hook, task)
	require.NoError(t, err)
	require.NotNil(t, req)

	// The framework request delivers the first recipient.
	assert.Equal(t, feishuAPIBaseURL+"/open-apis/im/v1/messages?receive_id_type=email", req.URL.String())
	assert.Equal(t, "Bearer fake-token", req.Header.Get("Authorization"))
	assert.Contains(t, string(reqBody), `"receive_id":"first@example.com"`)

	// Remaining recipients are delivered via direct Open API calls.
	assert.Equal(t, []string{"second@example.com", "third@example.com"}, sent)
}
