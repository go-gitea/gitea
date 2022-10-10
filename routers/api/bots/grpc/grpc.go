// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package grpc

import (
	"context"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/bots"

	"gitea.com/gitea/proto-go/ping/v1/pingv1connect"
	"gitea.com/gitea/proto-go/runner/v1/runnerv1connect"
	"github.com/bufbuild/connect-go"
	grpcreflect "github.com/bufbuild/connect-grpcreflect-go"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// RouteFn gRPC route registration
type RouteFn func() (string, http.Handler)

var compress1KB = connect.WithCompressMinBytes(1024)

var allServices = []string{
	runnerv1connect.RunnerServiceName,
	pingv1connect.PingServiceName,
	grpc_health_v1.Health_ServiceDesc.ServiceName,
}

func V1Route() (string, http.Handler) {
	// grpcV1
	return grpcreflect.NewHandlerV1(
		grpcreflect.NewStaticReflector(allServices...),
		compress1KB,
	)
}

func V1AlphaRoute() (string, http.Handler) {
	// grpcV1Alpha
	return grpcreflect.NewHandlerV1Alpha(
		grpcreflect.NewStaticReflector(allServices...),
		compress1KB,
	)
}

var withRunner = connect.WithInterceptors(connect.UnaryInterceptorFunc(func(unaryFunc connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		if methodName(request) == "Register" {
			return unaryFunc(ctx, request)
		}
		uuid := request.Header().Get("X-Runner-Token") // TODO: shouldn't be X-Runner-Token, maybe X-Runner-UUID
		// TODO: get runner from db, refuse request if it doesn't exist
		r := &bots.Runner{
			UUID: uuid,
		}
		ctx = context.WithValue(ctx, runnerCtxKey{}, r)
		return unaryFunc(ctx, request)
	}
}))

func methodName(req connect.AnyRequest) string {
	splits := strings.Split(req.Spec().Procedure, "/")
	if len(splits) > 0 {
		return splits[len(splits)-1]
	}
	return ""
}

type runnerCtxKey struct{}

func GetRunner(ctx context.Context) *bots.Runner {
	if v := ctx.Value(runnerCtxKey{}); v != nil {
		if r, ok := v.(*bots.Runner); ok {
			return r
		}
	}
	return nil
}
