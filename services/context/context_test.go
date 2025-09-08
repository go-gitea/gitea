// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestRemoveSessionCookieHeader(t *testing.T) {
	w := httptest.NewRecorder()
	w.Header().Add("Set-Cookie", (&http.Cookie{Name: setting.SessionConfig.CookieName, Value: "foo"}).String())
	w.Header().Add("Set-Cookie", (&http.Cookie{Name: "other", Value: "bar"}).String())
	assert.Len(t, w.Header().Values("Set-Cookie"), 2)
	removeSessionCookieHeader(w)
	assert.Len(t, w.Header().Values("Set-Cookie"), 1)
	assert.Contains(t, "other=bar", w.Header().Get("Set-Cookie"))
}

func TestRedirectToCurrentSite(t *testing.T) {
	setting.IsInTesting = true
	defer test.MockVariableValue(&setting.AppURL, "http://localhost:3000/sub/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()
	cases := []struct {
		location string
		want     string
	}{
		{"/", "/sub/"},
		{"http://localhost:3000/sub?k=v", "http://localhost:3000/sub?k=v"},
		{"http://other", "/sub/"},
	}
	for _, c := range cases {
		t.Run(c.location, func(t *testing.T) {
			req := &http.Request{URL: &url.URL{Path: "/"}}
			resp := httptest.NewRecorder()
			base := NewBaseContextForTest(resp, req)
			ctx := NewWebContext(base, nil, nil)
			ctx.RedirectToCurrentSite(c.location)
			redirect := test.RedirectURL(resp)
			assert.Equal(t, c.want, redirect)
		})
	}
}
