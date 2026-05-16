// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestFetchRedirectDelegate(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "https://gitea/")()

	cases := []struct {
		method string
		input  string
		status int
	}{
		{method: "POST", input: "/foo?k=v", status: http.StatusSeeOther},
		{method: "GET", input: "/foo?k=v", status: http.StatusBadRequest},
		{method: "POST", input: `\/foo?k=v`, status: http.StatusBadRequest},
		{method: "POST", input: `\\/foo?k=v`, status: http.StatusBadRequest},
		{method: "POST", input: "https://gitea/xxx", status: http.StatusSeeOther},
		{method: "POST", input: "https://other/xxx", status: http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.input, func(t *testing.T) {
			resp := httptest.NewRecorder()
			req := httptest.NewRequest(c.method, "/?redirect="+url.QueryEscape(c.input), nil)
			FetchRedirectDelegate(resp, req)
			assert.Equal(t, c.status, resp.Code)
			if c.status == http.StatusSeeOther {
				assert.Equal(t, c.input, resp.Header().Get("Location"))
			} else {
				assert.Empty(t, resp.Header().Get("Location"))
				assert.Equal(t, "Bad Request", strings.TrimSpace(resp.Body.String()))
			}
		})
	}
}
