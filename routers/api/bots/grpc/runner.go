// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package grpc

import (
	"net/http"

	"code.gitea.io/gitea/routers/api/bots/runner"
	"code.gitea.io/gitea/routers/api/bots/scheduler/queue"
	"gitea.com/gitea/proto-go/runner/v1/runnerv1connect"
)

func RunnerRoute() (string, http.Handler) {
	runnerService := &runner.Service{
		Scheduler: queue.New(),
	}

	return runnerv1connect.NewRunnerServiceHandler(
		runnerService,
		compress1KB,
	)
}
