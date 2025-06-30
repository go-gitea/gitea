// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pwn

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

const passwordURL = "https://api.pwnedpasswords.com/range/"

// ErrEmptyPassword is an empty password error
var ErrEmptyPassword = errors.New("password cannot be empty")

// Client is a HaveIBeenPwned client
type Client struct {
	ctx  context.Context
	http *http.Client
}

// New returns a new HaveIBeenPwned Client
func New(options ...ClientOption) *Client {
	client := &Client{
		ctx:  context.Background(),
		http: http.DefaultClient,
	}

	for _, opt := range options {
		opt(client)
	}

	return client
}

// ClientOption is a way to modify a new Client
type ClientOption func(*Client)

// WithHTTP will set the http.Client of a Client
func WithHTTP(httpClient *http.Client) func(pwnClient *Client) {
	return func(pwnClient *Client) {
		pwnClient.http = httpClient
	}
}

// WithContext will set the context.Context of a Client
func WithContext(ctx context.Context) func(pwnClient *Client) {
	return func(pwnClient *Client) {
		pwnClient.ctx = ctx
	}
}

func newRequest(ctx context.Context, method, url string, body io.ReadCloser) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", "Gitea "+setting.AppVer)
	return req, nil
}

// CheckPassword returns the number of times a password has been compromised
// Adding padding will make requests more secure, however is also slower
// because artificial responses will be added to the response
// For more information, see https://www.troyhunt.com/enhancing-pwned-passwords-privacy-with-padding/
func (c *Client) CheckPassword(pw string, padding bool) (int, error) {
	if pw == "" {
		return -1, ErrEmptyPassword
	}

	sha := sha1.New()
	sha.Write([]byte(pw))
	enc := hex.EncodeToString(sha.Sum(nil))
	prefix, suffix := enc[:5], enc[5:]

	req, err := newRequest(c.ctx, http.MethodGet, fmt.Sprintf("%s%s", passwordURL, prefix), nil)
	if err != nil {
		return -1, nil
	}
	if padding {
		req.Header.Add("Add-Padding", "true")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return -1, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	for pair := range strings.SplitSeq(string(body), "\n") {
		parts := strings.Split(pair, ":")
		if len(parts) != 2 {
			continue
		}
		if strings.EqualFold(suffix, parts[0]) {
			count, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
			if err != nil {
				return -1, err
			}
			return int(count), nil
		}
	}
	return 0, nil
}
