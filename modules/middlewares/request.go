// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middlewares

import (
	"net/http"
	"strings"
)

// IsAPIPath returns true if the specified URL is an API path
func IsAPIPath(req *http.Request) bool {
	return strings.HasPrefix(req.URL.Path, "/api/")
}

// IsInternalPath returns true if the specified URL is an internal API path
func IsInternalPath(req *http.Request) bool {
	return strings.HasPrefix(req.URL.Path, "/api/internal/")
}
