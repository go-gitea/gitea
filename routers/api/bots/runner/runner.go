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
	git_model "code.gitea.io/gitea/models/git"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/bots"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	bot_service "code.gitea.io/gitea/services/bots"
	secret_service "code.gitea.io/gitea/services/secrets"

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
	_ context.Context,
	_ *connect.Request[runnerv1.UpdateRunnerRequest],
) (*connect.Response[runnerv1.UpdateRunnerResponse], error) {
	// FIXME: we don't need it any longer
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
		AgentLabels:  req.Msg.AgentLabels,
		CustomLabels: req.Msg.CustomLabels,
	}
	if err := runner.GenerateToken(); err != nil {
		return nil, errors.New("can't generate token")
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
	if t, ok, err := pickTask(ctx, runner); err != nil {
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

func toCommitStatus(status bots_model.Status) api.CommitStatusState {
	switch status {
	case bots_model.StatusSuccess:
		return api.CommitStatusSuccess
	case bots_model.StatusFailure, bots_model.StatusCancelled, bots_model.StatusSkipped:
		return api.CommitStatusFailure
	case bots_model.StatusWaiting, bots_model.StatusBlocked:
		return api.CommitStatusPending
	case bots_model.StatusRunning:
		return api.CommitStatusRunning
	default:
		return api.CommitStatusError
	}
}

// UpdateTask updates the task status.
func (s *Service) UpdateTask(
	ctx context.Context,
	req *connect.Request[runnerv1.UpdateTaskRequest],
) (*connect.Response[runnerv1.UpdateTaskResponse], error) {
	{
		// to debug strange runner behaviors, it could be removed if all problems have been solved.
		stateMsg, _ := json.Marshal(req.Msg.State)
		log.Trace("update task with state: %s", stateMsg)
	}

	// Get Task first
	task, err := bots_model.GetTaskByID(ctx, req.Msg.State.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't find the task: %v", err)
	}
	if task.Result == runnerv1.Result_RESULT_CANCELLED {
		return connect.NewResponse(&runnerv1.UpdateTaskResponse{
			State: &runnerv1.TaskState{
				Id:     req.Msg.State.Id,
				Result: task.Result,
			},
		}), nil
	}

	task, err = bots_model.UpdateTaskByState(req.Msg.State)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update task: %v", err)
	}

	if err := task.LoadJob(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "load job: %v", err)
	}
	if err := task.Job.LoadAttributes(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "load run: %v", err)
	}

	if task.Job.Run.Event == webhook.HookEventPush {
		payload, err := task.Job.Run.GetPushEventPayload()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "GetPushEventPayload: %v", err)
		}

		creator, err := user_model.GetUserByID(payload.Pusher.ID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "GetUserByID: %v", err)
		}

		if err := git_model.NewCommitStatus(git_model.NewCommitStatusOptions{
			Repo:    task.Job.Run.Repo,
			SHA:     payload.HeadCommit.ID,
			Creator: creator,
			CommitStatus: &git_model.CommitStatus{
				SHA:         payload.HeadCommit.ID,
				TargetURL:   task.Job.Run.HTMLURL(),
				Description: "",
				Context:     task.Job.Name,
				CreatorID:   payload.Pusher.ID,
				State:       toCommitStatus(task.Job.Status),
			},
		}); err != nil {
			log.Error("Update commit status failed: %v", err)
		}
	}

	if req.Msg.State.Result != runnerv1.Result_RESULT_UNSPECIFIED {
		if err := bot_service.EmitJobsIfReady(task.Job.RunID); err != nil {
			log.Error("Emit ready jobs of run %d: %v", task.Job.RunID, err)
		}
	}

	return connect.NewResponse(&runnerv1.UpdateTaskResponse{}), nil
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

func pickTask(ctx context.Context, runner *bots_model.Runner) (*runnerv1.Task, bool, error) {
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
		"event":                  event,
		"run_id":                 fmt.Sprint(t.Job.ID),
		"run_number":             fmt.Sprint(t.Job.Run.Index),
		"run_attempt":            fmt.Sprint(t.Job.Attempt),
		"actor":                  fmt.Sprint(t.Job.Run.TriggerUser.Name),
		"repository":             fmt.Sprint(t.Job.Run.Repo.OwnerName) + "/" + fmt.Sprint(t.Job.Run.Repo.Name),
		"event_name":             fmt.Sprint(t.Job.Run.Event.Event()),
		"sha":                    fmt.Sprint(t.Job.Run.CommitSHA),
		"ref":                    fmt.Sprint(t.Job.Run.Ref),
		"ref_name":               "",
		"ref_type":               "",
		"head_ref":               "",
		"base_ref":               "",
		"token":                  t.Token,
		"repository_owner":       fmt.Sprint(t.Job.Run.Repo.OwnerName),
		"retention_days":         "",
		"gitea_default_bots_url": setting.Bots.DefaultBotsURL,
	})
	secrets := getSecretsOfTask(ctx, t)
	if _, ok := secrets["GITHUB_TOKEN"]; !ok {
		secrets["GITHUB_TOKEN"] = t.Token
	}
	if _, ok := secrets["GITEA_TOKEN"]; !ok {
		secrets["GITEA_TOKEN"] = t.Token
	}

	task := &runnerv1.Task{
		Id:              t.ID,
		WorkflowPayload: t.Job.WorkflowPayload,
		Context:         taskContext,
		Secrets:         secrets,
	}
	return task, true, nil
}

func getSecretsOfTask(ctx context.Context, task *bots_model.Task) map[string]string {
	// Returning an error is worse than returning empty secrets.

	secrets := map[string]string{}

	userSecrets, err := secret_service.FindUserSecrets(ctx, task.Job.Run.Repo.OwnerID)
	if err != nil {
		log.Error("find user secrets of %v: %v", task.Job.Run.Repo.OwnerID, err)
		// go on
	}
	repoSecrets, err := secret_service.FindRepoSecrets(ctx, task.Job.Run.RepoID)
	if err != nil {
		log.Error("find repo secrets of %v: %v", task.Job.Run.RepoID, err)
		// go on
	}

	// FIXME: Not sure if it's the exact meaning of secret.PullRequest
	pullRequest := task.Job.Run.Event == webhook.HookEventPullRequest

	for _, secret := range append(userSecrets, repoSecrets...) {
		if !pullRequest || secret.PullRequest {
			if v, err := secret_service.DecryptString(secret.Data); err != nil {
				log.Error("decrypt secret %v %q: %v", secret.ID, secret.Name, err)
				// go on
			} else {
				secrets[secret.Name] = v
			}
		}
	}
	return secrets
}
