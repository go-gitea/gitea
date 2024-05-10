// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
)

// RedirectURL returns the redirect URL of a http response.
// It also works for JSONRedirect: `{"redirect": "..."}`
func RedirectURL(resp http.ResponseWriter) string {
	loc := resp.Header().Get("Location")
	if loc != "" {
		return loc
	}
	if r, ok := resp.(*httptest.ResponseRecorder); ok {
		m := map[string]any{}
		err := json.Unmarshal(r.Body.Bytes(), &m)
		if err == nil {
			if loc, ok := m["redirect"].(string); ok {
				return loc
			}
		}
	}
	return ""
}

func IsNormalPageCompleted(s string) bool {
	return strings.Contains(s, `<footer class="page-footer"`) && strings.Contains(s, `</html>`)
}

func MockVariableValue[T any](p *T, v T) (reset func()) {
	old := *p
	*p = v
	return func() { *p = old }
}

type MockedOsExiter struct {
	exitCode int
	called   bool
}

func (m *MockedOsExiter) Exit(code int) {
	m.called = true
	m.exitCode = code
}

func (m *MockedOsExiter) FetchCode() int {
	c := util.Iif(m.called, m.exitCode, -1)
	m.exitCode = 0
	m.called = false
	return c
}
