// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/typesniffer"
)

type ServeHeaderOptions struct {
	ContentType        string // defaults to "application/octet-stream"
	ContentTypeCharset string
	ContentLength      *int64
	Disposition        string // defaults to "attachment"
	Filename           string
	CacheDuration      time.Duration // defaults to 5 minutes
	LastModified       time.Time
}

// SetServeHeaders sets necessary content serve headers
func (ctx *Context) SetServeHeaders(opts *ServeHeaderOptions) {
	header := ctx.Resp.Header()

	contentType := typesniffer.ApplicationOctetStream
	if opts.ContentType != "" {
		if opts.ContentTypeCharset != "" {
			contentType = opts.ContentType + "; charset=" + strings.ToLower(opts.ContentTypeCharset)
		} else {
			contentType = opts.ContentType
		}
	}
	header.Set("Content-Type", contentType)
	header.Set("X-Content-Type-Options", "nosniff")

	if opts.ContentLength != nil {
		header.Set("Content-Length", strconv.FormatInt(*opts.ContentLength, 10))
	}

	if opts.Filename != "" {
		disposition := opts.Disposition
		if disposition == "" {
			disposition = "attachment"
		}

		backslashEscapedName := strings.ReplaceAll(strings.ReplaceAll(opts.Filename, `\`, `\\`), `"`, `\"`) // \ -> \\, " -> \"
		header.Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, disposition, backslashEscapedName, url.PathEscape(opts.Filename)))
		header.Set("Access-Control-Expose-Headers", "Content-Disposition")
	}

	duration := opts.CacheDuration
	if duration == 0 {
		duration = 5 * time.Minute
	}
	httpcache.SetCacheControlInHeader(header, duration)

	if !opts.LastModified.IsZero() {
		header.Set("Last-Modified", opts.LastModified.UTC().Format(http.TimeFormat))
	}
}

// ServeContent serves content to http request
func (ctx *Context) ServeContent(r io.ReadSeeker, opts *ServeHeaderOptions) {
	ctx.SetServeHeaders(opts)
	http.ServeContent(ctx.Resp, ctx.Req, opts.Filename, opts.LastModified, r)
}
