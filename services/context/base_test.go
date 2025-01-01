// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestRedirect(t *testing.T) {
	setting.IsInTesting = true
	req, _ := http.NewRequest("GET", "/", nil)

	cases := []struct {
		url  string
		keep bool
	}{
		{"http://test", false},
		{"https://test", false},
		{"//test", false},
		{"/://test", true},
		{"/test", true},
	}
	for _, c := range cases {
		resp := httptest.NewRecorder()
		b := NewBaseContextForTest(resp, req)
		resp.Header().Add("Set-Cookie", (&http.Cookie{Name: setting.SessionConfig.CookieName, Value: "dummy"}).String())
		b.Redirect(c.url)
		has := resp.Header().Get("Set-Cookie") == "i_like_gitea=dummy"
		assert.Equal(t, c.keep, has, "url = %q", c.url)
	}

	req, _ = http.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()
	req.Header.Add("HX-Request", "true")
	b := NewBaseContextForTest(resp, req)
	b.Redirect("/other")
	assert.Equal(t, "/other", resp.Header().Get("HX-Redirect"))
	assert.Equal(t, http.StatusNoContent, resp.Code)
}
