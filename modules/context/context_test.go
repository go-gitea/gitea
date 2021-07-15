// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoteAddrNoHeader(t *testing.T) {
	expected := "123.456.78.9"
	req, _ := http.NewRequest(http.MethodGet, "url", nil)
	req.RemoteAddr = expected

	ctx := context.Context{Req: req}

	assert.Equal(t, expected, ctx.RemoteAddr(), "RemoteAddr should match the expected response")
}

func TestRemoteAddrXRealIpHeader(t *testing.T) {
	expected := "123.456.78.9"
	req, _ := http.NewRequest(http.MethodGet, "url", nil)
	req.Header.Add("X-Real-IP", expected)

	ctx := context.Context{Req: req}

	assert.Equal(t, expected, ctx.RemoteAddr(), "RemoteAddr should match the expected response")
}

func TestRemoteAddrXForwardedForHeader(t *testing.T) {
	expected := "123.456.78.9"
	req, _ := http.NewRequest(http.MethodGet, "url", nil)
	req.Header.Add("X-Forwarded-For", expected)

	ctx := context.Context{Req: req}

	assert.Equal(t, expected, ctx.RemoteAddr(), "RemoteAddr should match the expected response")
}
