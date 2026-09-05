// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/json"
)

// feishuAPIBaseURL is the default base URL of the Feishu (Lark) Open API. It
// is a variable so tests can point it at a mock server. Each webhook may
// override this via its URL field (e.g. for Lark Suite).
var feishuAPIBaseURL = "https://open.feishu.cn"

// feishuBaseURLFromWebhook returns the API base URL to use for a given
// webhook. Falls back to the global default when the webhook URL is empty.
func feishuBaseURLFromWebhook(w *webhook_model.Webhook) string {
	if w.URL != "" {
		return w.URL
	}
	return feishuAPIBaseURL
}

// feishuGetTenantAccessTokenFunc obtains a tenant_access_token for the app. It
// is a variable so tests can substitute a mock implementation. It returns the
// token and its lifetime in seconds as reported by the Feishu API.
var feishuGetTenantAccessTokenFunc = feishuGetTenantAccessToken

// feishuGetAccessTokenFunc returns a valid tenant_access_token for the app,
// fetching one from Feishu only when no unexpired token is cached. It is a
// variable so tests can substitute a mock implementation.
var feishuGetAccessTokenFunc = feishuGetAccessToken

// feishuSendMessageFunc delivers a single Feishu direct message. It is a
// variable so tests can substitute a mock implementation.
var feishuSendMessageFunc = feishuSendMessage

// feishuTokenEntry caches a tenant_access_token together with its expiry.
type feishuTokenEntry struct {
	token     string
	expiresAt time.Time
}

// feishuTokenMu guards feishuTokenCache.
var feishuTokenMu sync.Mutex

// feishuTokenCache caches the tenant_access_token per app (keyed by app_id) so
// we do not hit the Feishu token endpoint on every message delivery. The
// expiry is derived from the expire field returned by the token API.
var feishuTokenCache = map[string]feishuTokenEntry{}

// feishuTokenSafetyMargin is subtracted from the token's lifetime so we never
// attempt to use a token in its final moments before expiry.
const feishuTokenSafetyMargin = 5 * time.Minute

// feishuGetAccessToken returns a usable tenant_access_token for the given app,
// reusing a cached, still-valid token when possible. A new token is fetched
// only when none has been obtained yet or the cached one has (almost) expired.
func feishuGetAccessToken(ctx context.Context, baseURL, appID, appSecret string) (string, error) {
	feishuTokenMu.Lock()
	if entry, ok := feishuTokenCache[appID]; ok && time.Now().Before(entry.expiresAt) {
		token := entry.token
		feishuTokenMu.Unlock()
		return token, nil
	}
	feishuTokenMu.Unlock()

	token, expire, err := feishuGetTenantAccessTokenFunc(ctx, baseURL, appID, appSecret)
	if err != nil {
		return "", err
	}

	feishuTokenMu.Lock()
	defer feishuTokenMu.Unlock()
	// Another goroutine may have fetched and cached a token while we were
	// waiting on the token endpoint. Use the cached one if still valid to
	// avoid overwriting a potentially newer token.
	if entry, ok := feishuTokenCache[appID]; ok && time.Now().Before(entry.expiresAt) {
		return entry.token, nil
	}
	if expire > 0 {
		feishuTokenCache[appID] = feishuTokenEntry{
			token:     token,
			expiresAt: time.Now().Add(time.Duration(expire)*time.Second - feishuTokenSafetyMargin),
		}
	}
	return token, nil
}

// feishuSendMessage sends a text direct message to a single Feishu user via the
// Open API, using the provided tenant access token.
func feishuSendMessage(ctx context.Context, baseURL, token, receiveID, text string) error {
	contentBytes, _ := json.Marshal(map[string]string{"text": text})
	contentStr := string(contentBytes)
	body := map[string]string{
		"receive_id": receiveID,
		"msg_type":   "text",
		"content":    contentStr,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/open-apis/im/v1/messages?receive_id_type=email", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp, err := http.DefaultClient.Do(req.WithContext(cctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var r struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	if r.Code != 0 {
		return fmt.Errorf("feishu send message error: %d %s", r.Code, r.Msg)
	}
	return nil
}

// feishuGetTenantAccessToken requests a tenant_access_token for the app at the
// given API base URL. It returns the token and its lifetime in seconds (as
// reported by the Feishu API) so the caller can set an accurate cache expiry.
func feishuGetTenantAccessToken(ctx context.Context, baseURL, appID, appSecret string) (string, int, error) {
	reqBody := map[string]string{"app_id": appID, "app_secret": appSecret}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/open-apis/auth/v3/tenant_access_token/internal/", bytes.NewReader(b))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}
	// The tenant_access_token endpoint returns the token at the top level
	// (not wrapped in a "data" object like most other Feishu Open APIs).
	var r struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		AppAccessToken    string `json:"app_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return "", 0, err
	}
	if r.Code != 0 {
		return "", 0, fmt.Errorf("feishu token error: %d %s", r.Code, r.Msg)
	}
	// Only the tenant_access_token can be used to send Open API messages
	// (e.g. im/v1/messages); the app_access_token is not valid for that, so we
	// must not fall back to it here.
	if r.TenantAccessToken == "" {
		return "", 0, fmt.Errorf("feishu token error: empty tenant_access_token (code=%d msg=%s)", r.Code, r.Msg)
	}
	return r.TenantAccessToken, r.Expire, nil
}

// newFeishuNoopRequest builds a request that validates the app credentials
// against the token endpoint without delivering any direct message. It is used
// when there are no recipients to notify, so the framework still records a
// successful delivery (and surfaces misconfigured credentials).
func newFeishuNoopRequest(ctx context.Context, baseURL, appID, appSecret string) (*http.Request, []byte, error) {
	body := map[string]string{"app_id": appID, "app_secret": appSecret}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/open-apis/auth/v3/tenant_access_token/internal/", bytes.NewReader(b))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, b, nil
}
