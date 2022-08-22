// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package runner

import (
	"context"

	"code.gitea.io/gitea/modules/log"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"github.com/bufbuild/connect-go"
)

type Service struct{}

func (s *Service) Connect(
	ctx context.Context,
	req *connect.Request[runnerv1.ConnectRequest],
) (*connect.Response[runnerv1.ConnectResponse], error) {
	log.Info("Request headers: %v", req.Header())
	res := connect.NewResponse(&runnerv1.ConnectResponse{
		Stage: &runnerv1.Stage{
			RunnerUuid: "foobar",
			BuildUuid:  "foobar",
		},
	})
	res.Header().Set("Gitea-Version", "runnerv1")
	return res, nil
}

func (s *Service) Accept(
	ctx context.Context,
	req *connect.Request[runnerv1.AcceptRequest],
) (*connect.Response[runnerv1.AcceptResponse], error) {
	log.Info("Request headers: %v", req.Header())
	res := connect.NewResponse(&runnerv1.AcceptResponse{
		JobId: 100,
	})
	res.Header().Set("Gitea-Version", "runnerv1")
	return res, nil
}
