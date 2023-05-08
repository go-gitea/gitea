// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// RemoteAddr returns the client machine ip address
func (ctx *Context) RemoteAddr() string {
	return ctx.Req.RemoteAddr
}

// Params returns the param on route
func (ctx *Context) Params(p string) string {
	s, _ := url.PathUnescape(chi.URLParam(ctx.Req, strings.TrimPrefix(p, ":")))
	return s
}

// ParamsInt64 returns the param on route as int64
func (ctx *Context) ParamsInt64(p string) int64 {
	v, _ := strconv.ParseInt(ctx.Params(p), 10, 64)
	return v
}

// SetParams set params into routes
func (ctx *Context) SetParams(k, v string) {
	chiCtx := chi.RouteContext(ctx)
	chiCtx.URLParams.Add(strings.TrimPrefix(k, ":"), url.PathEscape(v))
}

// UploadStream returns the request body or the first form file
// Only form files need to get closed.
func (ctx *Context) UploadStream() (rd io.ReadCloser, needToClose bool, err error) {
	contentType := strings.ToLower(ctx.Req.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") || strings.HasPrefix(contentType, "multipart/form-data") {
		if err := ctx.Req.ParseMultipartForm(32 << 20); err != nil {
			return nil, false, err
		}
		if ctx.Req.MultipartForm.File == nil {
			return nil, false, http.ErrMissingFile
		}
		for _, files := range ctx.Req.MultipartForm.File {
			if len(files) > 0 {
				r, err := files[0].Open()
				return r, true, err
			}
		}
		return nil, false, http.ErrMissingFile
	}
	return ctx.Req.Body, false, nil
}
