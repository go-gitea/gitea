// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web"
	pingv1 "gitea.com/gitea/proto-go/ping/v1"
	"gitea.com/gitea/proto-go/ping/v1/pingv1connect"

	"github.com/bufbuild/connect-go"
	grpchealth "github.com/bufbuild/connect-grpchealth-go"
	grpcreflect "github.com/bufbuild/connect-grpcreflect-go"
)

type PingService struct{}

func (s *PingService) Ping(
	ctx context.Context,
	req *connect.Request[pingv1.PingRequest],
) (*connect.Response[pingv1.PingResponse], error) {
	log.Info("Content-Type: %s", req.Header().Get("Content-Type"))
	log.Info("User-Agent: %s", req.Header().Get("User-Agent"))
	log.Info("X-Gitea-Token: %s", req.Header().Get("X-Gitea-Token"))
	res := connect.NewResponse(&pingv1.PingResponse{
		Data: fmt.Sprintf("Hello, %s!", req.Msg.Data),
	})
	res.Header().Set("Gitea-Version", "v1")
	return res, nil
}

func pingServiceRoute(r *web.Route) {
	compress1KB := connect.WithCompressMinBytes(1024)

	pingService := &PingService{}
	connectPath, connecthandler := pingv1connect.NewPingServiceHandler(
		pingService,
		compress1KB,
	)

	// grpcV1
	grpcPath, gHandler := grpcreflect.NewHandlerV1(
		grpcreflect.NewStaticReflector(pingv1connect.PingServiceName),
		compress1KB,
	)

	// grpcV1Alpha
	grpcAlphaPath, gAlphaHandler := grpcreflect.NewHandlerV1Alpha(
		grpcreflect.NewStaticReflector(pingv1connect.PingServiceName),
		compress1KB,
	)

	// grpcHealthCheck
	grpcHealthPath, gHealthHandler := grpchealth.NewHandler(
		grpchealth.NewStaticChecker(pingv1connect.PingServiceName),
		compress1KB,
	)

	r.Post(connectPath+"{name}", grpcHandler(connecthandler))
	r.Post(grpcPath+"{name}", grpcHandler(gHandler))
	r.Post(grpcAlphaPath+"{name}", grpcHandler(gAlphaHandler))
	r.Post(grpcHealthPath+"{name}", grpcHandler(gHealthHandler))
}
