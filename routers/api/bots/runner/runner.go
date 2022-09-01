// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package runner

import (
	"context"
	"errors"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/modules/log"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
	"gitea.com/gitea/proto-go/runner/v1/runnerv1connect"

	"github.com/bufbuild/connect-go"
)

type Service struct {
	runnerv1connect.UnimplementedRunnerServiceHandler
}

// Register for new runner.
func (s *Service) Register(
	ctx context.Context,
	req *connect.Request[runnerv1.RegisterRequest],
) (*connect.Response[runnerv1.RegisterResponse], error) {
	log.Info("Request headers: %v", req.Header())

	token := req.Header().Get("X-Runner-Token")
	log.Info("token: %v", token)

	if token == "" {
		return nil, errors.New("missing runner token")
	}

	runner, err := bots_model.GetRunnerByToken(token)
	if err != nil {
		return nil, errors.New("runner not found")
	}

	// update runner information
	runner.Arch = req.Msg.Arch
	runner.OS = req.Msg.Os
	runner.Capacity = req.Msg.Capacity
	if err := bots_model.UpdateRunner(ctx, runner, []string{"arch", "os", "capacity"}...); err != nil {
		return nil, errors.New("can't update runner")
	}

	res := connect.NewResponse(&runnerv1.RegisterResponse{
		Runner: &runnerv1.Runner{
			Uuid:     runner.UUID,
			Os:       req.Msg.Os,
			Arch:     req.Msg.Arch,
			Capacity: req.Msg.Capacity,
		},
	})

	return res, nil
}

// Request requests the next available build stage for execution.
func (s *Service) Request(
	ctx context.Context,
	req *connect.Request[runnerv1.RequestRequest],
) (*connect.Response[runnerv1.RequestResponse], error) {
	res := connect.NewResponse(&runnerv1.RequestResponse{
		Stage: &runnerv1.Stage{},
	})
	return res, nil
}

// Update updates the build stage.
func (s *Service) Update(
	ctx context.Context,
	req *connect.Request[runnerv1.UpdateRequest],
) (*connect.Response[runnerv1.UpdateResponse], error) {
	res := connect.NewResponse(&runnerv1.UpdateResponse{})
	return res, nil
}

// UpdateStep updates the build step.
func (s *Service) UpdateStep(
	ctx context.Context,
	req *connect.Request[runnerv1.UpdateStepRequest],
) (*connect.Response[runnerv1.UpdateStepResponse], error) {
	res := connect.NewResponse(&runnerv1.UpdateStepResponse{})
	return res, nil
}
