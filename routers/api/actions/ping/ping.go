// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ping

import (
	"context"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/log"

	pingv1 "code.gitea.io/actions-proto-go/ping/v1"
	"code.gitea.io/actions-proto-go/ping/v1/pingv1connect"
	"connectrpc.com/connect"
)

func NewPingServiceHandler() (string, http.Handler) {
	return pingv1connect.NewPingServiceHandler(&Service{})
}

var _ pingv1connect.PingServiceHandler = (*Service)(nil)

type Service struct{}

func (s *Service) Ping(
	ctx context.Context,
	req *connect.Request[pingv1.PingRequest],
) (*connect.Response[pingv1.PingResponse], error) {
	log.Trace("Content-Type: %s", req.Header().Get("Content-Type"))
	log.Trace("User-Agent: %s", req.Header().Get("User-Agent"))
	res := connect.NewResponse(&pingv1.PingResponse{
		Data: fmt.Sprintf("Hello, %s!", req.Msg.Data),
	})
	return res, nil
}
