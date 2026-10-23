// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"context"
	"crypto/tls"
	"net/http"
	"testing"

	"codenest.dev/modules/reqctx"
	"codenest.dev/modules/setting"
	"codenest.dev/modules/test"
	"codenest.dev/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestIsRelativeURL(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "http://localhost:3000/sub/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()
	rel := []string{
		"",
		"foo",
		"/",
		"/foo?k=%20#abc",
		"/foo?k=\\",
	}
	for _, s := range rel {
		assert.True(t, IsRelativeURL(s), "rel = %q", s)
	}
	abs := []string{
		"//",
		"\\\\",
		"/\\",
		"\\/",
		"/a/../\\b",
		"/any\\thing",
		"mailto:a@b.com",
		"https://test.com",
	}
	for _, s := range abs {
		assert.False(t, IsRelativeURL(s), "abs = %q", s)
	}
}

func testCtxWithPublicURL(t *testing.T, req *http.Request, supportPublicURL ...bool) context.Context {
	reqCtx := reqctx.NewRequestContextForTest(t)
	_ = RequestWithContext(req, reqCtx)
	if util.OptionalArg(supportPublicURL, true) {
		MarkRequestSupportPublicURL(reqCtx)
	}
	return reqCtx
}

func TestGuessCurrentHostURL(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "http://cfg-host/sub/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()
	headersWithProto := http.Header{"X-Forwarded-Proto": {"https"}}
	maliciousProtoHeaders := http.Header{"X-Forwarded-Proto": {"http://attacker.host/?trash="}}

	t.Run("Legacy", func(t *testing.T) {
		defer test.MockVariableValue(&setting.PublicURLDetection, setting.PublicURLLegacy)()

		assert.Equal(t, "http://cfg-host", GuessCurrentHostURL(t.Context()))

		// legacy: "Host" is not used when there is no "X-Forwarded-Proto" header
		ctx := testCtxWithPublicURL(t, &http.Request{Host: "req-host:3000"})
		assert.Equal(t, "http://cfg-host", GuessCurrentHostURL(ctx))

		// if "X-Forwarded-Proto" exists, then use it and "Host" header
		ctx = testCtxWithPublicURL(t, &http.Request{Host: "req-host:3000", Header: headersWithProto})
		assert.Equal(t, "https://req-host:3000", GuessCurrentHostURL(ctx))

		ctx = testCtxWithPublicURL(t, &http.Request{Host: "req-host:3000", Header: maliciousProtoHeaders})
		assert.Equal(t, "http://cfg-host", GuessCurrentHostURL(ctx))
	})

	t.Run("Auto", func(t *testing.T) {
		defer test.MockVariableValue(&setting.PublicURLDetection, setting.PublicURLAuto)()

		assert.Equal(t, "http://cfg-host", GuessCurrentHostURL(t.Context()))

		// auto: always use "Host" header, the scheme is determined by "X-Forwarded-Proto" header, or TLS config if no "X-Forwarded-Proto" header
		ctx := testCtxWithPublicURL(t, &http.Request{Host: "req-host:3000"})
		assert.Equal(t, "http://req-host:3000", GuessCurrentHostURL(ctx))

		ctx = testCtxWithPublicURL(t, &http.Request{Host: "req-host", TLS: &tls.ConnectionState{}})
		assert.Equal(t, "https://req-host", GuessCurrentHostURL(ctx))

		ctx = testCtxWithPublicURL(t, &http.Request{Host: "req-host:3000", Header: headersWithProto})
		assert.Equal(t, "https://req-host:3000", GuessCurrentHostURL(ctx))

		ctx = testCtxWithPublicURL(t, &http.Request{Host: "req-host:3000", Header: maliciousProtoHeaders})
		assert.Equal(t, "http://req-host:3000", GuessCurrentHostURL(ctx))

		ctx = testCtxWithPublicURL(t, &http.Request{Host: "req-host:3000", Header: maliciousProtoHeaders}, false)
		assert.Equal(t, "http://cfg-host", GuessCurrentHostURL(ctx))
	})

	t.Run("Never", func(t *testing.T) {
		defer test.MockVariableValue(&setting.PublicURLDetection, setting.PublicURLNever)()

		assert.Equal(t, "http://cfg-host", GuessCurrentHostURL(t.Context()))

		ctx := testCtxWithPublicURL(t, &http.Request{Host: "req-host:3000"})
		assert.Equal(t, "http://cfg-host", GuessCurrentHostURL(ctx))

		ctx = testCtxWithPublicURL(t, &http.Request{Host: "req-host:3000", TLS: &tls.ConnectionState{}})
		assert.Equal(t, "http://cfg-host", GuessCurrentHostURL(ctx))

		ctx = testCtxWithPublicURL(t, &http.Request{Host: "req-host:3000", Header: headersWithProto})
		assert.Equal(t, "http://cfg-host", GuessCurrentHostURL(ctx))
	})
}

func TestMakeAbsoluteURL(t *testing.T) {
	defer test.MockVariableValue(&setting.Protocol, "http")()
	defer test.MockVariableValue(&setting.AppURL, "http://cfg-host/sub/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()

	ctx := t.Context()
	assert.Equal(t, "http://cfg-host/sub/", MakeAbsoluteURL(ctx, ""))
	assert.Equal(t, "http://cfg-host/foo", MakeAbsoluteURL(ctx, "foo"))
	assert.Equal(t, "http://cfg-host/foo", MakeAbsoluteURL(ctx, "/foo"))
	assert.Equal(t, "http://other/foo", MakeAbsoluteURL(ctx, "http://other/foo"))

	ctx = testCtxWithPublicURL(t, &http.Request{
		Host: "user-host",
	})
	assert.Equal(t, "http://cfg-host/foo", MakeAbsoluteURL(ctx, "/foo"))

	ctx = testCtxWithPublicURL(t, &http.Request{
		Host: "user-host",
		Header: map[string][]string{
			"X-Forwarded-Host": {"forwarded-host"},
		},
	})
	assert.Equal(t, "http://cfg-host/foo", MakeAbsoluteURL(ctx, "/foo"))

	ctx = testCtxWithPublicURL(t, &http.Request{
		Host: "user-host",
		Header: map[string][]string{
			"X-Forwarded-Host":  {"forwarded-host"},
			"X-Forwarded-Proto": {"https"},
		},
	})
	assert.Equal(t, "https://user-host/foo", MakeAbsoluteURL(ctx, "/foo"))
}

func TestIsCurrentCodeNestSiteURL(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "http://localhost:3000/sub/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()
	ctx := t.Context()
	good := []string{
		"?key=val",
		"/sub",
		"/sub/",
		"/sub/foo",
		"/sub/foo/",
		"http://localhost:3000/sub?key=val",
		"http://localhost:3000/sub/",
	}
	for _, s := range good {
		assert.True(t, IsCurrentCodeNestSiteURL(ctx, s), "good = %q", s)
	}
	bad := []string{
		".",
		"foo",
		"/",
		"//",
		"\\\\",
		"/foo",
		"http://localhost:3000/sub/..",
		"http://localhost:3000/other",
		"http://other/",
	}
	for _, s := range bad {
		assert.False(t, IsCurrentCodeNestSiteURL(ctx, s), "bad = %q", s)
	}

	setting.AppURL = "http://localhost:3000/"
	setting.AppSubURL = ""
	assert.False(t, IsCurrentCodeNestSiteURL(ctx, "//"))
	assert.False(t, IsCurrentCodeNestSiteURL(ctx, "\\\\"))
	assert.False(t, IsCurrentCodeNestSiteURL(ctx, "http://localhost"))
	assert.True(t, IsCurrentCodeNestSiteURL(ctx, "http://localhost:3000?key=val"))

	ctx = testCtxWithPublicURL(t, &http.Request{
		Host: "user-host",
		Header: map[string][]string{
			"X-Forwarded-Host":  {"forwarded-host"},
			"X-Forwarded-Proto": {"https"},
		},
	})
	assert.True(t, IsCurrentCodeNestSiteURL(ctx, "http://localhost:3000"))
	assert.True(t, IsCurrentCodeNestSiteURL(ctx, "https://user-host"))
	assert.False(t, IsCurrentCodeNestSiteURL(ctx, "https://forwarded-host"))
}

func TestParseCodeNestSiteURL(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "http://localhost:3000/sub/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()
	ctx := t.Context()
	tests := []struct {
		url string
		exp *CodeNestSiteURL
	}{
		{"http://localhost:3000/sub?k=v", &CodeNestSiteURL{RoutePath: ""}},
		{"http://localhost:3000/sub/", &CodeNestSiteURL{RoutePath: ""}},
		{"http://localhost:3000/sub/foo", &CodeNestSiteURL{RoutePath: "/foo"}},
		{"http://localhost:3000/sub/foo/bar", &CodeNestSiteURL{RoutePath: "/foo/bar", OwnerName: "foo", RepoName: "bar"}},
		{"http://localhost:3000/sub/foo/bar/", &CodeNestSiteURL{RoutePath: "/foo/bar", OwnerName: "foo", RepoName: "bar"}},
		{"http://localhost:3000/sub/attachments/bar", &CodeNestSiteURL{RoutePath: "/attachments/bar"}},
		{"http://localhost:3000/other", nil},
		{"http://other/", nil},
	}
	for _, test := range tests {
		su := ParseCodeNestSiteURL(ctx, test.url)
		assert.Equal(t, test.exp, su, "URL = %s", test.url)
	}
}
