// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"net/http"
)

// RedirectURL returns the redirect URL of a http response.
func RedirectURL(resp http.ResponseWriter) string {
	return resp.Header().Get("Location")
}
