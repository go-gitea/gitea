// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"net/http"
	"strings"
)

// RedirectURL returns the redirect URL of a http response.
func RedirectURL(resp http.ResponseWriter) string {
	return resp.Header().Get("Location")
}

func IsNormalPageCompleted(s string) bool {
	return strings.Contains(s, `<footer class="page-footer"`) && strings.Contains(s, `</html>`)
}

func MockVariableValue[T any](p *T, v T) (reset func()) {
	old := *p
	*p = v
	return func() { *p = old }
}
