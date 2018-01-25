// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package test

import (
	"net/http"
)

// RedirectURL returns the redirect URL of a http response.
func RedirectURL(resp http.ResponseWriter) string {
	return resp.Header().Get("Location")
}
