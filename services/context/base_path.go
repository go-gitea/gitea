// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/url"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"github.com/go-chi/chi/v5"
)

// PathParam returns the param in request path, eg: "/{var}" => "/a%2fb", then `var == "a/b"`
func (b *Base) PathParam(name string) string {
	s, err := url.PathUnescape(b.PathParamRaw(name))
	if err != nil && !setting.IsProd {
		panic("Failed to unescape path param: " + err.Error() + ", there seems to be a double-unescaping bug")
	}
	return s
}

// PathParamRaw returns the raw param in request path, eg: "/{var}" => "/a%2fb", then `var == "a%2fb"`
func (b *Base) PathParamRaw(name string) string {
	// this check should be removed before 1.25 release
	if strings.HasPrefix(name, ":") {
		panic("path param should not start with ':'")
	}
	return chi.URLParam(b.Req, name)
}

// PathParamInt64 returns the param in request path as int64
func (b *Base) PathParamInt64(p string) int64 {
	v, _ := strconv.ParseInt(b.PathParam(p), 10, 64)
	return v
}

// SetPathParam set request path params into routes
func (b *Base) SetPathParam(k, v string) {
	// this check should be removed before 1.25 release
	if strings.HasPrefix(k, ":") {
		panic("path param should not start with ':'")
	}
	chi.RouteContext(b).URLParams.Add(k, url.PathEscape(v))
}
