// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web"
	v1 "gitea.com/gitea/proto/gen/proto/v1"
	"gitea.com/gitea/proto/gen/proto/v1/v1connect"

	"github.com/bufbuild/connect-go"
	grpchealth "github.com/bufbuild/connect-grpchealth-go"
	grpcreflect "github.com/bufbuild/connect-grpcreflect-go"
)

type RunnerService struct{}

func (s *RunnerService) Connect(
	ctx context.Context,
	req *connect.Request[v1.ConnectRequest],
) (*connect.Response[v1.ConnectResponse], error) {
	log.Info("Request headers: %v", req.Header())
	res := connect.NewResponse(&v1.ConnectResponse{
		JobId: 100,
	})
	res.Header().Set("Gitea-Version", "v1")
	return res, nil
}

func (s *RunnerService) Accept(
	ctx context.Context,
	req *connect.Request[v1.AcceptRequest],
) (*connect.Response[v1.AcceptResponse], error) {
	log.Info("Request headers: %v", req.Header())
	res := connect.NewResponse(&v1.AcceptResponse{
		JobId: 100,
	})
	res.Header().Set("Gitea-Version", "v1")
	return res, nil
}

func runnerServiceRoute(r *web.Route) {
	compress1KB := connect.WithCompressMinBytes(1024)

	runnerService := &RunnerService{}
	connectPath, connecthandler := v1connect.NewRunnerServiceHandler(
		runnerService,
		compress1KB,
	)

	// grpcV1
	grpcPath, gHandler := grpcreflect.NewHandlerV1(
		grpcreflect.NewStaticReflector(v1connect.RunnerServiceName),
		compress1KB,
	)

	// grpcV1Alpha
	grpcAlphaPath, gAlphaHandler := grpcreflect.NewHandlerV1Alpha(
		grpcreflect.NewStaticReflector(v1connect.RunnerServiceName),
		compress1KB,
	)

	// grpcHealthCheck
	grpcHealthPath, gHealthHandler := grpchealth.NewHandler(
		grpchealth.NewStaticChecker(v1connect.RunnerServiceName),
		compress1KB,
	)

	r.Post(connectPath+"{name}", grpcHandler(connecthandler))
	r.Post(grpcPath+"{name}", grpcHandler(gHandler))
	r.Post(grpcAlphaPath+"{name}", grpcHandler(gAlphaHandler))
	r.Post(grpcHealthPath+"{name}", grpcHandler(gHealthHandler))
}
