// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"net/http"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/bots/grpc"
)

func grpcHandler(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Trace("protocol version: %v", r.Proto)
		h.ServeHTTP(w, r)
	}
}

func gRPCRouter(r *web.Route, fn grpc.RouteFn) {
	p, h := fn()
	r.Post(p+"{name}", grpcHandler(h))
}

func Routes(r *web.Route) *web.Route {
	gRPCRouter(r, grpc.V1Route)
	gRPCRouter(r, grpc.V1AlphaRoute)
	gRPCRouter(r, grpc.HealthRoute)
	gRPCRouter(r, grpc.PingRoute)
	gRPCRouter(r, grpc.RunnerRoute)

	return r
}
