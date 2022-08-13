// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"code.gitea.io/gitea/modules/web"
	"gitea.com/gitea/proto/gen/proto/v1/v1connect"

	"github.com/bufbuild/connect-go"
	grpchealth "github.com/bufbuild/connect-grpchealth-go"
	grpcreflect "github.com/bufbuild/connect-grpcreflect-go"
)

func Routes(r *web.Route) {
	compress1KB := connect.WithCompressMinBytes(1024)

	service := &RunnerService{}
	path, handler := v1connect.NewBuildServiceHandler(
		service,
		compress1KB,
	)

	// grpcV1
	grpcPath, gHandler := grpcreflect.NewHandlerV1(
		grpcreflect.NewStaticReflector(v1connect.BuildServiceName),
		compress1KB,
	)

	// grpcV1Alpha
	grpcAlphaPath, gAlphaHandler := grpcreflect.NewHandlerV1Alpha(
		grpcreflect.NewStaticReflector(v1connect.BuildServiceName),
		compress1KB,
	)

	// grpcHealthCheck
	grpcHealthPath, gHealthHandler := grpchealth.NewHandler(
		grpchealth.NewStaticChecker(v1connect.BuildServiceName),
		compress1KB,
	)

	// socket connection
	r.Get("/socket", socketServe)
	// restful connection
	r.Post(path+"{name}", giteaHandler(handler))
	// grpc connection
	r.Post(grpcPath+"{name}", giteaHandler(gHandler))
	r.Post(grpcAlphaPath+"{name}", giteaHandler(gAlphaHandler))
	// healthy check connection
	r.Post(grpcHealthPath+"{name}", giteaHandler(gHealthHandler))
}
