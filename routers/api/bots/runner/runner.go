// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package runner

import (
	"context"
	"errors"

	"code.gitea.io/gitea/core"
	bots_model "code.gitea.io/gitea/models/bots"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
	"gitea.com/gitea/proto-go/runner/v1/runnerv1connect"

	"github.com/bufbuild/connect-go"
)

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
	log.Debug("manager: request queue item")

	stage, err := s.Scheduler.Request(ctx, core.Filter{
		Kind: req.Msg.Kind,
		Type: req.Msg.Type,
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

	res := connect.NewResponse(&runnerv1.RequestResponse{
		Stage: stage,
	})
	return res, nil
}

// Details fetches build details
func (s *Service) Detail(
	ctx context.Context,
	req *connect.Request[runnerv1.DetailRequest],
) (*connect.Response[runnerv1.DetailResponse], error) {
	log.Info("stag id %d", req.Msg.Stage.Id)

	// fetch stage data
	stage, err := bots_model.GetStageByID(req.Msg.Stage.Id)
	if err != nil {
		return nil, err
	}

	stage.Machine = req.Msg.Stage.Machine
	stage.Status = core.StatusPending

	count, err := bots_model.UpdateBuildStage(stage, "machine", "status")
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, core.ErrDataLock
	}

	// fetch build data
	build, err := bots_model.GetBuildByID(stage.BuildID)
	if err != nil {
		return nil, err
	}

	// fetch repo data
	repo, err := repo_model.GetRepositoryByID(build.RepoID)
	if err != nil {
		return nil, err
	}

	res := connect.NewResponse(&runnerv1.DetailResponse{
		Stage: &runnerv1.Stage{
			Id:      stage.ID,
			BuildId: stage.BuildID,
			Name:    stage.Name,
			Kind:    stage.Kind,
			Type:    stage.Type,
			Status:  string(stage.Status),
			Started: int64(stage.Started),
			Stopped: int64(stage.Stopped),
			Machine: stage.Machine,
		},
		Build: &runnerv1.Build{
			Id:   build.ID,
			Name: build.Name,
		},
		Repo: &runnerv1.Repo{
			Id:   repo.ID,
			Name: repo.Name,
		},
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
