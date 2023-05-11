// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"io"
	"net/http"

	"code.gitea.io/gitea/modules/httplib"
)

type ServeHeaderOptions httplib.ServeHeaderOptions

func (ctx *Context) SetServeHeaders(opt *ServeHeaderOptions) {
	httplib.ServeSetHeaders(ctx.Resp, (*httplib.ServeHeaderOptions)(opt))
}

// ServeContent serves content to http request
func (ctx *Context) ServeContent(r io.ReadSeeker, opts *ServeHeaderOptions) {
	httplib.ServeSetHeaders(ctx.Resp, (*httplib.ServeHeaderOptions)(opts))
	http.ServeContent(ctx.Resp, ctx.Req, opts.Filename, opts.LastModified, r)
}
