// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package runner

import (
	"context"
	"errors"

	"code.gitea.io/gitea/core"
	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/modules/log"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
	"gitea.com/gitea/proto-go/runner/v1/runnerv1connect"

	"github.com/bufbuild/connect-go"
)

var _ runnerv1connect.RunnerServiceClient = (*Service)(nil)

type Service struct {
	Scheduler core.Scheduler

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

	// TODO: Get token data from runner_token table
	runner, err := bots_model.GetRunnerByToken(token)
	if err != nil {
		return nil, errors.New("runner not found")
	}

	// update runner information
	runner.AgentLabels = req.Msg.AgentLabels
	runner.CustomLabels = req.Msg.CustomLabels
	runner.Name = req.Msg.Name
	if err := bots_model.UpdateRunner(ctx, runner, []string{"name", "agent_labels", "custom_labels"}...); err != nil {
		return nil, errors.New("can't update runner")
	}

	res := connect.NewResponse(&runnerv1.RegisterResponse{
		Runner: &runnerv1.Runner{
			Uuid:  runner.UUID,
			Token: runner.Token,
		},
	})

	return res, nil
}

// Request requests the next available build stage for execution.
func (s *Service) FetchTask(
	ctx context.Context,
	req *connect.Request[runnerv1.FetchTaskRequest],
) (*connect.Response[runnerv1.FetchTaskResponse], error) {
	log.Debug("manager: request queue item")

	task, err := s.Scheduler.Request(ctx, core.Filter{
		OS:   req.Msg.Os,
		Arch: req.Msg.Arch,
	})
	if err != nil && ctx.Err() != nil {
		log.Debug("manager: context canceled")
		return nil, err
	}
	if err != nil {
		log.Warn("manager: request queue item error")
		return nil, err
	}

	// TODO: update task and check data lock
	task.Machine = req.Msg.Os

	res := connect.NewResponse(&runnerv1.FetchTaskResponse{
		Task: task,
	})
	return res, nil
}

// UpdateTask updates the task status.
func (s *Service) UpdateTask(
	ctx context.Context,
	req *connect.Request[runnerv1.UpdateTaskRequest],
) (*connect.Response[runnerv1.UpdateTaskResponse], error) {
	res := connect.NewResponse(&runnerv1.UpdateTaskResponse{})
	return res, nil
}

// UpdateLog uploads log of the task.
func (s *Service) UpdateLog(
	ctx context.Context,
	req *connect.Request[runnerv1.UpdateLogRequest],
) (*connect.Response[runnerv1.UpdateLogResponse], error) {
	res := connect.NewResponse(&runnerv1.UpdateLogResponse{})
	return res, nil
}
