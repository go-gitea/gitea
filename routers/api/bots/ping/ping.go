// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ping

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/log"

	pingv1 "code.gitea.io/bots-proto-go/ping/v1"
	"code.gitea.io/bots-proto-go/ping/v1/pingv1connect"
	"github.com/bufbuild/connect-go"
)

type Service struct {
	pingv1connect.UnimplementedPingServiceHandler
}

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
