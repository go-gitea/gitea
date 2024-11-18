// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"context"
	"crypto/tls"
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

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
	}
	for _, s := range rel {
		assert.True(t, IsRelativeURL(s), "rel = %q", s)
	}
	abs := []string{
		"//",
		"\\\\",
		"/\\",
		"\\/",
		"mailto:a@b.com",
		"https://test.com",
	}
	for _, s := range abs {
		assert.False(t, IsRelativeURL(s), "abs = %q", s)
	}
}

func TestGuessCurrentHostURL(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "http://cfg-host/sub/")()
	defer test.MockVariableValue(&setting.LocalURL, "http://localhost:3000/sub/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()

	ctx := context.Background()
	assert.Equal(t, "http://cfg-host", GuessCurrentHostURL(ctx))

	ctx = context.WithValue(ctx, RequestContextKey, &http.Request{
		Host: "localhost:3000",
	})
	assert.Equal(t, "http://localhost:3000", GuessCurrentHostURL(ctx))

	ctx = context.WithValue(ctx, RequestContextKey, &http.Request{
		Host: "localhost:3000",
		TLS:  &tls.ConnectionState{},
	})
	assert.Equal(t, "http://cfg-host", GuessCurrentHostURL(ctx))

	defer test.MockVariableValue(&setting.LocalURL, "https://localhost/sub/")()
	ctx = context.WithValue(ctx, RequestContextKey, &http.Request{
		Host: "localhost",
		TLS:  &tls.ConnectionState{},
	})
	assert.Equal(t, "https://localhost", GuessCurrentHostURL(ctx))
}

func TestMakeAbsoluteURL(t *testing.T) {
	defer test.MockVariableValue(&setting.Protocol, "http")()
	defer test.MockVariableValue(&setting.AppURL, "http://cfg-host/sub/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()

	ctx := context.Background()
	assert.Equal(t, "http://cfg-host/sub/", MakeAbsoluteURL(ctx, ""))
	assert.Equal(t, "http://cfg-host/foo", MakeAbsoluteURL(ctx, "foo"))
	assert.Equal(t, "http://cfg-host/foo", MakeAbsoluteURL(ctx, "/foo"))
	assert.Equal(t, "http://other/foo", MakeAbsoluteURL(ctx, "http://other/foo"))

	ctx = context.WithValue(ctx, RequestContextKey, &http.Request{
		Host: "user-host",
	})
	assert.Equal(t, "http://cfg-host/foo", MakeAbsoluteURL(ctx, "/foo"))

	ctx = context.WithValue(ctx, RequestContextKey, &http.Request{
		Host: "user-host",
		Header: map[string][]string{
			"X-Forwarded-Host": {"forwarded-host"},
		},
	})
	assert.Equal(t, "http://cfg-host/foo", MakeAbsoluteURL(ctx, "/foo"))

	ctx = context.WithValue(ctx, RequestContextKey, &http.Request{
		Host: "user-host",
		Header: map[string][]string{
			"X-Forwarded-Host":  {"forwarded-host"},
			"X-Forwarded-Proto": {"https"},
		},
	})
	assert.Equal(t, "https://user-host/foo", MakeAbsoluteURL(ctx, "/foo"))
}

func TestIsCurrentGiteaSiteURL(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "http://localhost:3000/sub/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()
	ctx := context.Background()
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
		assert.True(t, IsCurrentGiteaSiteURL(ctx, s), "good = %q", s)
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
		assert.False(t, IsCurrentGiteaSiteURL(ctx, s), "bad = %q", s)
	}

	setting.AppURL = "http://localhost:3000/"
	setting.AppSubURL = ""
	assert.False(t, IsCurrentGiteaSiteURL(ctx, "//"))
	assert.False(t, IsCurrentGiteaSiteURL(ctx, "\\\\"))
	assert.False(t, IsCurrentGiteaSiteURL(ctx, "http://localhost"))
	assert.True(t, IsCurrentGiteaSiteURL(ctx, "http://localhost:3000?key=val"))

	ctx = context.WithValue(ctx, RequestContextKey, &http.Request{
		Host: "user-host",
		Header: map[string][]string{
			"X-Forwarded-Host":  {"forwarded-host"},
			"X-Forwarded-Proto": {"https"},
		},
	})
	assert.True(t, IsCurrentGiteaSiteURL(ctx, "http://localhost:3000"))
	assert.True(t, IsCurrentGiteaSiteURL(ctx, "https://user-host"))
	assert.False(t, IsCurrentGiteaSiteURL(ctx, "https://forwarded-host"))
}
