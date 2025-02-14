// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pwn

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockTransport struct{}

func (mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != "api.pwnedpasswords.com" {
		return nil, errors.New("unsupported host")
	}
	respMap := map[string]string{
		"/range/5c1d8": "EAF2F254732680E8AC339B84F3266ECCBB5:1\r\nFC446EB88938834178CB9322C1EE273C2A7:2",
		"/range/ba189": "FD4CB34F0378BCB15D23F6FFD28F0775C9E:3\r\nFDF342FCD8C3611DAE4D76E8A992A3E4169:4",
		"/range/a1733": "C4CE0F1F0062B27B9E2F41AF0C08218017C:1\r\nFC446EB88938834178CB9322C1EE273C2A7:2\r\nFE81480327C992FE62065A827429DD1318B:0",
		"/range/5617b": "FD4CB34F0378BCB15D23F6FFD28F0775C9E:3\r\nFDF342FCD8C3611DAE4D76E8A992A3E4169:4\r\nFE81480327C992FE62065A827429DD1318B:0",
		"/range/79082": "FDF342FCD8C3611DAE4D76E8A992A3E4169:4\r\nFE81480327C992FE62065A827429DD1318B:0\r\nAFEF386F56EB0B4BE314E07696E5E6E6536:0",
	}
	if resp, ok := respMap[req.URL.Path]; ok {
		return &http.Response{Request: req, Body: io.NopCloser(strings.NewReader(resp))}, nil
	}
	return nil, errors.New("unsupported path")
}

func TestPassword(t *testing.T) {
	client := New(WithHTTP(&http.Client{Transport: mockTransport{}}))

	count, err := client.CheckPassword("", false)
	assert.ErrorIs(t, err, ErrEmptyPassword, "blank input should return ErrEmptyPassword")
	assert.Equal(t, -1, count)

	count, err = client.CheckPassword("pwned", false)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	count, err = client.CheckPassword("notpwned", false)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	count, err = client.CheckPassword("paddedpwned", true)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	count, err = client.CheckPassword("paddednotpwned", true)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	count, err = client.CheckPassword("paddednotpwnedzero", true)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}
