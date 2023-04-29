// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

type mockResponseWriter struct {
	header http.Header
}

func (m *mockResponseWriter) Header() http.Header {
	return m.header
}

func (m *mockResponseWriter) Write(bytes []byte) (int, error) {
	panic("implement me")
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	panic("implement me")
}

func TestRemoveSessionCookieHeader(t *testing.T) {
	w := &mockResponseWriter{}
	w.header = http.Header{}
	w.header.Add("Set-Cookie", (&http.Cookie{Name: setting.SessionConfig.CookieName, Value: "foo"}).String())
	w.header.Add("Set-Cookie", (&http.Cookie{Name: "other", Value: "bar"}).String())
	assert.Len(t, w.Header().Values("Set-Cookie"), 2)
	removeSessionCookieHeader(w)
	assert.Len(t, w.Header().Values("Set-Cookie"), 1)
	assert.Contains(t, "other=bar", w.Header().Get("Set-Cookie"))
}
