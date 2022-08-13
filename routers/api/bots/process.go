// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"net/http"

	"code.gitea.io/gitea/modules/log"
	v1 "gitea.com/gitea/proto/gen/proto/v1"

	"github.com/bufbuild/connect-go"
)

type RunnerService struct{}

func (s *RunnerService) Connect(
	ctx context.Context,
	req *connect.Request[v1.ConnectRequest],
) (*connect.Response[v1.ConnectResponse], error) {
	log.Info("Request headers: %v", req.Header())
	res := connect.NewResponse(&v1.ConnectResponse{
		JobId: 100,
	})
	res.Header().Set("Gitea-Version", "v1")
	return res, nil
}

func (s *RunnerService) Accept(
	ctx context.Context,
	req *connect.Request[v1.AcceptRequest],
) (*connect.Response[v1.AcceptResponse], error) {
	log.Info("Request headers: %v", req.Header())
	res := connect.NewResponse(&v1.AcceptResponse{
		JobId: 100,
	})
	res.Header().Set("Gitea-Version", "v1")
	return res, nil
}

func giteaHandler(h http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info("Got connection: %v", r.Proto)
		h.ServeHTTP(w, r)
	})
}
