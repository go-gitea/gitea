// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package turnstile

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/internal/modules/json"
	"code.gitea.io/gitea/internal/modules/setting"
)

// Response is the structure of JSON returned from API
type Response struct {
	Success     bool        `json:"success"`
	ChallengeTS string      `json:"challenge_ts"`
	Hostname    string      `json:"hostname"`
	ErrorCodes  []ErrorCode `json:"error-codes"`
	Action      string      `json:"login"`
	Cdata       string      `json:"cdata"`
}

// Verify calls Cloudflare Turnstile API to verify token
func Verify(ctx context.Context, response string) (bool, error) {
	// Cloudflare turnstile official access instruction address: https://developers.cloudflare.com/turnstile/get-started/server-side-validation/
	post := url.Values{
		"secret":   {setting.Service.CfTurnstileSecret},
		"response": {response},
	}
	// Basically a copy of http.PostForm, but with a context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(post.Encode()))
	if err != nil {
		return false, fmt.Errorf("Failed to create CAPTCHA request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("Failed to send CAPTCHA response: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("Failed to read CAPTCHA response: %w", err)
	}

	var jsonResponse Response
	if err := json.Unmarshal(body, &jsonResponse); err != nil {
		return false, fmt.Errorf("Failed to parse CAPTCHA response: %w", err)
	}

	var respErr error
	if len(jsonResponse.ErrorCodes) > 0 {
		respErr = jsonResponse.ErrorCodes[0]
	}
	return jsonResponse.Success, respErr
}

// ErrorCode is a reCaptcha error
type ErrorCode string

// String fulfills the Stringer interface
func (e ErrorCode) String() string {
	switch e {
	case "missing-input-secret":
		return "The secret parameter was not passed."
	case "invalid-input-secret":
		return "The secret parameter was invalid or did not exist."
	case "missing-input-response":
		return "The response parameter was not passed."
	case "invalid-input-response":
		return "The response parameter is invalid or has expired."
	case "bad-request":
		return "The request was rejected because it was malformed."
	case "timeout-or-duplicate":
		return "The response parameter has already been validated before."
	case "internal-error":
		return "An internal error happened while validating the response. The request can be retried."
	}
	return string(e)
}

// Error fulfills the error interface
func (e ErrorCode) Error() string {
	return e.String()
}
