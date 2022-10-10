// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package grpc

import (
	"net/http"

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
