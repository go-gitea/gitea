// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package runner

import (
	"context"
	"errors"
	"fmt"
	"time"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/modules/bots"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
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
	runnerv1connect.UnimplementedRunnerServiceHandler
}

// UpdateRunner update runner status or other data.
func (s *Service) UpdateRunner(
	ctx context.Context,
	req *connect.Request[runnerv1.UpdateRunnerRequest],
) (*connect.Response[runnerv1.UpdateRunnerResponse], error) {
	runner := GetRunner(ctx)

	// check status
	if runner.Status == req.Msg.Status {
		return connect.NewResponse(&runnerv1.UpdateRunnerResponse{}), nil
	}

	// update status
	runner.Status = req.Msg.Status
	if err := bots_model.UpdateRunner(ctx, runner, "status"); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&runnerv1.UpdateRunnerResponse{}), nil
}

// Register for new runner.
func (s *Service) Register(
	ctx context.Context,
	req *connect.Request[runnerv1.RegisterRequest],
) (*connect.Response[runnerv1.RegisterResponse], error) {
	if req.Msg.Token == "" || req.Msg.Name == "" {
		return nil, errors.New("missing runner token, name")
	}

	runnerToken, err := bots_model.GetRunnerToken(req.Msg.Token)
	if err != nil {
		return nil, errors.New("runner token not found")
	}

	if runnerToken.IsActive {
		return nil, errors.New("runner token has already activated")
	}

	// create new runner
	runner := &bots_model.Runner{
		UUID:         gouuid.New().String(),
		Name:         req.Msg.Name,
		OwnerID:      runnerToken.OwnerID,
		RepoID:       runnerToken.RepoID,
		Token:        req.Msg.Token,
		Status:       runnerv1.RunnerStatus_RUNNER_STATUS_OFFLINE,
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
			Id:           runner.ID,
			Uuid:         runner.UUID,
			Token:        runner.Token,
			Name:         runner.Name,
			AgentLabels:  runner.AgentLabels,
			CustomLabels: runner.CustomLabels,
			Status:       runner.Status,
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
		log.Error("pick task failed: %v", err)
		return nil, status.Errorf(codes.Internal, "pick task: %v", err)
	} else if ok {
		task = t
	}

	// avoid crazy retry
	if task == nil {
		duration := 2 * time.Second
		if deadline, ok := ctx.Deadline(); ok {
			if d := time.Until(deadline) - time.Second; d < duration {
				duration = d
			}
		}
		time.Sleep(duration)
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

	if err := bots_model.UpdateTaskByState(req.Msg.State); err != nil {
		return nil, status.Errorf(codes.Internal, "update task: %v", err)
	}

	return res, nil
}

// UpdateLog uploads log of the task.
func (s *Service) UpdateLog(
	ctx context.Context,
	req *connect.Request[runnerv1.UpdateLogRequest],
) (*connect.Response[runnerv1.UpdateLogResponse], error) {
	res := connect.NewResponse(&runnerv1.UpdateLogResponse{})

	task, err := bots_model.GetTaskByID(ctx, req.Msg.TaskId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get task: %v", err)
	}
	ack := task.LogLength

	if len(req.Msg.Rows) == 0 || req.Msg.Index > ack || int64(len(req.Msg.Rows))+req.Msg.Index <= ack {
		res.Msg.AckIndex = ack
		return res, nil
	}

	if task.LogInStorage {
		return nil, status.Errorf(codes.AlreadyExists, "log file has been archived")
	}

	rows := req.Msg.Rows[ack-req.Msg.Index:]
	ns, err := bots.WriteLogs(ctx, task.LogFilename, task.LogSize, rows)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "write logs: %v", err)
	}
	task.LogLength += int64(len(rows))
	if task.LogIndexes == nil {
		task.LogIndexes = &bots_model.LogIndexes{}
	}
	for _, n := range ns {
		*task.LogIndexes = append(*task.LogIndexes, task.LogSize)
		task.LogSize += int64(n)
	}

	res.Msg.AckIndex = task.LogLength

	var remove func()
	if req.Msg.NoMore {
		task.LogInStorage = true
		remove, err = bots.TransferLogs(ctx, task.LogFilename)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "transfer logs: %v", err)
		}
	}

	if err := bots_model.UpdateTask(ctx, task, "log_indexes", "log_length", "log_size", "log_in_storage"); err != nil {
		return nil, status.Errorf(codes.Internal, "update task: %v", err)
	}
	if remove != nil {
		remove()
	}

	return res, nil
}

func (s *Service) pickTask(ctx context.Context, runner *bots_model.Runner) (*runnerv1.Task, bool, error) {
	t, ok, err := bots_model.CreateTaskForRunner(ctx, runner)
	if err != nil {
		return nil, false, fmt.Errorf("CreateTaskForRunner: %w", err)
	}
	if !ok {
		return nil, false, nil
	}

	event := map[string]interface{}{}
	_ = json.Unmarshal([]byte(t.Job.Run.EventPayload), &event)

	// TODO: more context in https://docs.github.com/cn/actions/learn-github-actions/contexts#github-context
	taskContext, _ := structpb.NewStruct(map[string]interface{}{
		"event":            event,
		"run_id":           fmt.Sprint(t.Job.ID),
		"run_number":       fmt.Sprint(t.Job.Run.Index),
		"run_attempt":      fmt.Sprint(t.Job.Attempt),
		"actor":            fmt.Sprint(t.Job.Run.TriggerUser.Name),
		"repository":       fmt.Sprint(t.Job.Run.Repo.Name),
		"event_name":       fmt.Sprint(t.Job.Run.Event.Event()),
		"sha":              fmt.Sprint(t.Job.Run.CommitSHA),
		"ref":              fmt.Sprint(t.Job.Run.Ref),
		"ref_name":         "",
		"ref_type":         "",
		"head_ref":         "",
		"base_ref":         "",
		"token":            "",
		"repository_owner": fmt.Sprint(t.Job.Run.Repo.OwnerName),
		"retention_days":   "",
	})

	task := &runnerv1.Task{
		Id:              t.ID,
		WorkflowPayload: t.Job.WorkflowPayload,
		Context:         taskContext,
		Secrets:         nil, // TODO: query secrets
	}
	return task, true, nil
}
