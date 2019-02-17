// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routers

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

// Metrics validate auth token and render prometheus metrics
func Metrics(ctx *context.Context) {
	if setting.Metrics.Token == "" {
		promhttp.Handler().ServeHTTP(ctx.Resp, ctx.Req.Request)
		return
	}
	header := ctx.Req.Header.Get("Authorization")
	if header == "" {
		ctx.Error(401)
		return
	}
	if header != "Bearer "+setting.Metrics.Token {
		ctx.Error(401)
		return
	}
	promhttp.Handler().ServeHTTP(ctx.Resp, ctx.Req.Request)
}
