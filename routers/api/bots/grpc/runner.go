// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package grpc

import (
	"net/http"

	"code.gitea.io/gitea/routers/api/bots/runner"

	"code.gitea.io/bots-proto-go/runner/v1/runnerv1connect"
)

func RunnerRoute() (string, http.Handler) {
	runnerService := &runner.Service{}

	return runnerv1connect.NewRunnerServiceHandler(
		runnerService,
		compress1KB,
		runner.WithRunner,
	)
}
