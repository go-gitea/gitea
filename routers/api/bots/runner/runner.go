// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package runner

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"code.gitea.io/gitea/core"
	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
	"gitea.com/gitea/proto-go/runner/v1/runnerv1connect"

	"github.com/bufbuild/connect-go"
	gouuid "github.com/google/uuid"
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
	if req.Msg.Token == "" || req.Msg.Name == "" || req.Msg.Url == "" {
		return nil, errors.New("missing runner token, name or URL")
	}

	runnerToken, err := bots_model.GetRunnerToken(req.Msg.Token)
	if err != nil {
		return nil, errors.New("runner token not found")
	}

	if runnerToken.IsActive {
		return nil, errors.New("runner token has already activated")
	}

	// valiate user data
	u, err := url.Parse(req.Msg.Url)
	if err != nil {
		return nil, errors.New("can't parse url: " + req.Msg.Url)
	}

	urls := strings.Split(u.Path, "/")
	if runnerToken.OwnerID != 0 {
		if len(urls) < 2 {
			return nil, errors.New("can't parse owner name")
		}
		owner, err := user.GetUserByID(runnerToken.OwnerID)
		if err != nil {
			return nil, errors.New("can't get owner name")
		}
		if owner.LowerName != strings.ToLower(urls[1]) {
			return nil, errors.New("wrong owner name")
		}
	}

	if runnerToken.RepoID != 0 {
		if len(urls) < 3 {
			return nil, errors.New("can't parse repo name")
		}

		r, err := repo.GetRepositoryByIDCtx(ctx, runnerToken.RepoID)
		if err != nil {
			return nil, errors.New("can't get repo name")
		}

		if r.LowerName != strings.ToLower(urls[2]) {
			return nil, errors.New("wrong repo name")
		}
	}

	// create new runner
	runner := &bots_model.Runner{
		UUID:         gouuid.New().String(),
		Name:         req.Msg.Name,
		OwnerID:      runnerToken.OwnerID,
		RepoID:       runnerToken.RepoID,
		Token:        req.Msg.Token,
		Status:       core.StatusOffline,
		AgentLabels:  req.Msg.AgentLabels,
		CustomLabels: req.Msg.CustomLabels,
	}

	// create new runner
	if err := bots_model.NewRunner(ctx, runner); err != nil {
		return nil, errors.New("can't create new runner")
	}

	// update token status
	runnerToken.IsActive = true
	if err := bots_model.UpdateRunnerToken(ctx, runnerToken, "is_active"); err != nil {
		return nil, errors.New("can't update runner token status")
	}

	res := connect.NewResponse(&runnerv1.RegisterResponse{
		Runner: &runnerv1.Runner{
			Uuid:         runner.UUID,
			Token:        runner.Token,
			Name:         runner.Name,
			AgentLabels:  runner.AgentLabels,
			CustomLabels: runner.CustomLabels,
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
