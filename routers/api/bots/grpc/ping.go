// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package grpc

import (
	"net/http"

	"code.gitea.io/gitea/routers/api/bots/ping"
	"gitea.com/gitea/proto-go/ping/v1/pingv1connect"
)

func PingRoute() (string, http.Handler) {
	pingService := &ping.Service{}

	return pingv1connect.NewPingServiceHandler(
		pingService,
		compress1KB,
	)
}
