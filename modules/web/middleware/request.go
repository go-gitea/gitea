// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"context"
	"net/http"
	"strings"
)

// IsAPIPath returns true if the specified URL is an API path
func IsAPIPath(req *http.Request) bool {
	return strings.HasPrefix(req.URL.Path, "/api/")
}

type contextRequestKeyType struct{}

var contextRequestKey contextRequestKeyType

func WithContextRequest(c context.Context, req *http.Request) context.Context {
	return context.WithValue(c, contextRequestKey, req)
}

func GetContextRequest(c context.Context) *http.Request {
	req, _ := c.Value(contextRequestKey).(*http.Request)
	return req
}
