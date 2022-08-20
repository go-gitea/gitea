// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"code.gitea.io/gitea/modules/web"
	"gitea.com/gitea/proto-go/ping/v1/pingv1connect"
	"gitea.com/gitea/proto-go/runner/v1/runnerv1connect"

	"github.com/bufbuild/connect-go"
	grpchealth "github.com/bufbuild/connect-grpchealth-go"
)

func healthServiceRoute(r *web.Route) {
	compress1KB := connect.WithCompressMinBytes(1024)

	// grpcHealthCheck
	grpcHealthPath, gHealthHandler := grpchealth.NewHandler(
		grpchealth.NewStaticChecker(
			runnerv1connect.RunnerServiceName,
			pingv1connect.PingServiceName,
		),
		compress1KB,
	)

	r.Post(grpcHealthPath+"{name}", grpcHandler(gHealthHandler))
}
