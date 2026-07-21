// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// https://github.com/go-gitea/gitea/pull/37492#issuecomment-4355482585
// End users need it "since it is the only one open-source option in Gitea besides the image captcha"

package mcaptcha

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

func Verify(ctx context.Context, token string) (bool, error) {
	client := &Client{
		ServerURL: setting.Service.McaptchaURL,
		SiteKey:   setting.Service.McaptchaSitekey,
		Secret:    setting.Service.McaptchaSecret,
		Token:     token,
	}
	return client.Verify(ctx)
}

type Client struct {
	ServerURL string

	Secret  string
	SiteKey string
	Token   string
}

func (c *Client) Verify(ctx context.Context) (bool, error) {
	verifyURL := strings.TrimSuffix(c.ServerURL, "/") + "/api/v1/pow/siteverify"
	reqParams := map[string]string{"secret": c.Secret, "key": c.SiteKey, "token": c.Token}
	reqBody, _ := json.Marshal(reqParams)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyURL, bytes.NewReader(reqBody))
	if err != nil {
		return false, fmt.Errorf("unable to create verify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("unable to complete verify request: %w", err)
	}
	defer res.Body.Close()

	respContent, err := io.ReadAll(io.LimitReader(res.Body, 16*1024))
	if err != nil {
		return false, fmt.Errorf("unable to read response body: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected response content: %q", util.TruncateRunes(util.UnsafeBytesToString(respContent), 100))
	}

	var resp struct {
		Valid bool `json:"valid"`
	}
	err = json.Unmarshal(respContent, &resp)
	if err != nil {
		return false, fmt.Errorf("unable to decode response: %w", err)
	}
	return resp.Valid, nil
}
