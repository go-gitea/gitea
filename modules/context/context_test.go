// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestRemoveSessionCookieHeader(t *testing.T) {
	w := httplib.NewMockResponseWriter()
	w.Header().Add("Set-Cookie", (&http.Cookie{Name: setting.SessionConfig.CookieName, Value: "foo"}).String())
	w.Header().Add("Set-Cookie", (&http.Cookie{Name: "other", Value: "bar"}).String())
	assert.Len(t, w.Header().Values("Set-Cookie"), 2)
	removeSessionCookieHeader(w)
	assert.Len(t, w.Header().Values("Set-Cookie"), 1)
	assert.Contains(t, "other=bar", w.Header().Get("Set-Cookie"))
}
