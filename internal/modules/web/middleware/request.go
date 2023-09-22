// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"net/http"
	"strings"
)

// IsAPIPath returns true if the specified URL is an API path
func IsAPIPath(req *http.Request) bool {
	return strings.HasPrefix(req.URL.Path, "/api/")
}
