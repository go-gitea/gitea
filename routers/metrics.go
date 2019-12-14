// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routers

import (
	"crypto/subtle"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"

	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	got := []byte(header)
	want := []byte("Bearer " + setting.Metrics.Token)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		ctx.Error(401)
		return
	}
	promhttp.Handler().ServeHTTP(ctx.Resp, ctx.Req.Request)
}
