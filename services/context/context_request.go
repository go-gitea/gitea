// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"io"
	"net/http"
	"strings"
)

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
