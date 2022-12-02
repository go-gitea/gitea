// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bots

import (
	"context"
	"net/http"

	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/bots/grpc"
)

func Routes(_ context.Context, prefix string) *web.Route {
	m := web.NewRoute()

	for _, fn := range []grpc.RouteFn{
		grpc.V1Route,
		grpc.V1AlphaRoute,
		grpc.HealthRoute,
		grpc.PingRoute,
		grpc.RunnerRoute,
	} {
		path, handler := fn()
		m.Post(path+"*", http.StripPrefix(prefix, handler).ServeHTTP)
	}

	return m
}
