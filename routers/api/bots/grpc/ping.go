// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package grpc

import (
	"net/http"

	"code.gitea.io/gitea/routers/api/bots/ping"

	"code.gitea.io/bots-proto-go/ping/v1/pingv1connect"
)

func PingRoute() (string, http.Handler) {
	pingService := &ping.Service{}

	return pingv1connect.NewPingServiceHandler(
		pingService,
		compress1KB,
	)
}
