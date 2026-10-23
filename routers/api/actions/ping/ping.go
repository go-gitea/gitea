// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ping

import (
	"context"
	"fmt"
	"net/http"

	pingv1 "gitea.dev/actionslib/ping/v1"
	"gitea.dev/actionslib/ping/v1/pingv1connect"
	"gitea.dev/modules/log"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
)

func NewPingServiceHandler() (string, http.Handler) {
	server := connect.NewServer()
	pingv1connect.RegisterPingServiceHandler(server, &Service{})
	mux := http.NewServeMux()
	connecthttp.Mount(mux, server)
	return "/" + pingv1connect.PingServiceName + "/", mux
}

var _ pingv1connect.PingServiceHandler = (*Service)(nil)

type Service struct{}

func (s *Service) Ping(
	ctx context.Context,
	req *pingv1.PingRequest,
) (*pingv1.PingResponse, error) {
	info, _ := connect.CallInfoForServerContext(ctx)
	log.Trace("Content-Type: %s", info.RequestHeader().Get("Content-Type"))
	log.Trace("User-Agent: %s", info.RequestHeader().Get("User-Agent"))
	res := &pingv1.PingResponse{
		Data: fmt.Sprintf("Hello, %s!", req.Data),
	}
	return res, nil
}
