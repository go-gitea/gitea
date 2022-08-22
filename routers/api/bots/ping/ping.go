// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ping

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/log"
	pingv1 "gitea.com/gitea/proto-go/ping/v1"

	"github.com/bufbuild/connect-go"
)

type Service struct{}

func (s *Service) Ping(
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
