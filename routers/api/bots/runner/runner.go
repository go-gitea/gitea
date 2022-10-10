// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package runner

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"code.gitea.io/gitea/core"
	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
	"gitea.com/gitea/proto-go/runner/v1/runnerv1connect"

	"github.com/bufbuild/connect-go"
	gouuid "github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
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
	token := req.Header().Get("X-Runner-Token")

	if token == "" || req.Msg.Name == "" || req.Msg.Url == "" {
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

// FetchTask assigns a task to the runner
func (s *Service) FetchTask(
	ctx context.Context,
	req *connect.Request[runnerv1.FetchTaskRequest],
) (*connect.Response[runnerv1.FetchTaskResponse], error) {
	runner := GetRunner(ctx)

	var task *runnerv1.Task
	if t, ok, err := s.pickTask(ctx, runner); err != nil {
		return nil, status.Errorf(codes.Internal, "pick task: %v", err)
	} else if ok {
		task = t
	}

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

func (s *Service) pickTask(ctx context.Context, runner *bots_model.Runner) (*runnerv1.Task, bool, error) {
	t, job, run, ok, err := bots_model.CreateTask(runner)
	if err != nil {
		return nil, false, fmt.Errorf("CreateTask: %w", err)
	}
	if !ok {
		return nil, false, nil
	}

	event := map[string]interface{}{}
	_ = json.Unmarshal([]byte(run.EventPayload), &event)

	// TODO: more context in https://docs.github.com/cn/actions/learn-github-actions/contexts#github-context
	taskContext, _ := structpb.NewStruct(map[string]interface{}{
		"event":            event,
		"run_id":           fmt.Sprint(run.ID),
		"run_number":       fmt.Sprint(run.Index),
		"run_attempt":      fmt.Sprint(job.Attempt),
		"actor":            fmt.Sprint(run.TriggerUser.Name),
		"repository":       fmt.Sprint(run.Repo.Name),
		"event_name":       fmt.Sprint(run.Event.Event()),
		"sha":              fmt.Sprint(run.CommitSHA),
		"ref":              fmt.Sprint(run.Ref),
		"ref_name":         "",
		"ref_type":         "",
		"head_ref":         "",
		"base_ref":         "",
		"token":            "",
		"repository_owner": fmt.Sprint(run.Repo.OwnerName),
		"retention_days":   "",
	})

	task := &runnerv1.Task{
		Id:              t.ID,
		WorkflowPayload: job.WorkflowPayload,
		Context:         taskContext,
		Secrets:         nil, // TODO: query secrets
	}
	return task, true, nil
}
