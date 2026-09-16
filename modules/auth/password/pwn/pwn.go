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

	"gitea.dev/modules/httplib"
	"gitea.dev/modules/setting"
)

const (
	passwordURL     = "https://api.pwnedpasswords.com/range/"
	maxResponseSize = 1 << 20
)

// Client is a HaveIBeenPwned client
type Client struct {
	mockTransport http.RoundTripper
}

func New() *Client {
	return &Client{}
}

// CheckPassword returns the number of times a password has been compromised
// Adding padding will make requests more secure, however is also slower
// because artificial responses will be added to the response
// For more information, see https://www.troyhunt.com/enhancing-pwned-passwords-privacy-with-padding/
func (c *Client) CheckPassword(ctx context.Context, pw string, padding bool) (int64, error) {
	if pw == "" {
		return -1, errors.New("password cannot be empty")
	}

	sha := sha1.New()
	sha.Write([]byte(pw))
	enc := hex.EncodeToString(sha.Sum(nil))
	prefix, suffix := enc[:5], enc[5:]

	req := httplib.NewRequest(fmt.Sprintf("%s%s", passwordURL, prefix), http.MethodGet)
	req.SetContext(ctx).SetTransport(c.mockTransport)
	req.Header("User-Agent", "Gitea "+setting.AppVer)
	if padding {
		req.Header("Add-Padding", "true")
	}
	resp, err := req.Response()
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return -1, fmt.Errorf("unexpected status code %d from HIBP API", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return -1, err
	}
	if len(body) > maxResponseSize {
		return -1, fmt.Errorf("response from HIBP API exceeds %d bytes", maxResponseSize)
	}

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
			return count, nil
		}
	}
	return 0, nil
}
