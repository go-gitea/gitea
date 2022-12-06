// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"context"
	"errors"
	"fmt"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	actions_service "code.gitea.io/gitea/services/actions"
	secret_service "code.gitea.io/gitea/services/secrets"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"code.gitea.io/actions-proto-go/runner/v1/runnerv1connect"
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

// Register for new runner.
func (s *Service) Register(
	ctx context.Context,
	req *connect.Request[runnerv1.RegisterRequest],
) (*connect.Response[runnerv1.RegisterResponse], error) {
	if req.Msg.Token == "" || req.Msg.Name == "" {
		return nil, errors.New("missing runner token, name")
	}

	runnerToken, err := actions_model.GetRunnerToken(req.Msg.Token)
	if err != nil {
		return nil, errors.New("runner token not found")
	}

	if runnerToken.IsActive {
		return nil, errors.New("runner token has already activated")
	}

	// create new runner
	runner := &actions_model.BotRunner{
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
	if err := actions_model.NewRunner(ctx, runner); err != nil {
		return nil, errors.New("can't create new runner")
	}

	// update token status
	runnerToken.IsActive = true
	if err := actions_model.UpdateRunnerToken(ctx, runnerToken, "is_active"); err != nil {
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
	task, err := actions_model.GetTaskByID(ctx, req.Msg.State.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't find the task: %v", err)
	}
	if task.Status.IsCancelled() {
		return connect.NewResponse(&runnerv1.UpdateTaskResponse{
			State: &runnerv1.TaskState{
				Id:     req.Msg.State.Id,
				Result: task.Status.AsResult(),
			},
		}), nil
	}

	task, err = actions_model.UpdateTaskByState(req.Msg.State)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update task: %v", err)
	}

	if err := task.LoadJob(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "load job: %v", err)
	}

	if err := actions_service.CreateCommitStatus(ctx, task.Job); err != nil {
		log.Error("Update commit status failed: %v", err)
		// go on
	}

	if req.Msg.State.Result != runnerv1.Result_RESULT_UNSPECIFIED {
		if err := actions_service.EmitJobsIfReady(task.Job.RunID); err != nil {
			log.Error("Emit ready jobs of run %d: %v", task.Job.RunID, err)
		}
	}

	return connect.NewResponse(&runnerv1.UpdateTaskResponse{
		State: &runnerv1.TaskState{
			Id:     req.Msg.State.Id,
			Result: task.Status.AsResult(),
		},
	}), nil
}

// UpdateLog uploads log of the task.
func (s *Service) UpdateLog(
	ctx context.Context,
	req *connect.Request[runnerv1.UpdateLogRequest],
) (*connect.Response[runnerv1.UpdateLogResponse], error) {
	res := connect.NewResponse(&runnerv1.UpdateLogResponse{})

	task, err := actions_model.GetTaskByID(ctx, req.Msg.TaskId)
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
	ns, err := actions.WriteLogs(ctx, task.LogFilename, task.LogSize, rows)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "write logs: %v", err)
	}
	task.LogLength += int64(len(rows))
	if task.LogIndexes == nil {
		task.LogIndexes = &actions_model.LogIndexes{}
	}
	for _, n := range ns {
		*task.LogIndexes = append(*task.LogIndexes, task.LogSize)
		task.LogSize += int64(n)
	}

	res.Msg.AckIndex = task.LogLength

	var remove func()
	if req.Msg.NoMore {
		task.LogInStorage = true
		remove, err = actions.TransferLogs(ctx, task.LogFilename)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "transfer logs: %v", err)
		}
	}

	if err := actions_model.UpdateTask(ctx, task, "log_indexes", "log_length", "log_size", "log_in_storage"); err != nil {
		return nil, status.Errorf(codes.Internal, "update task: %v", err)
	}
	if remove != nil {
		remove()
	}

	return res, nil
}

func pickTask(ctx context.Context, runner *actions_model.BotRunner) (*runnerv1.Task, bool, error) {
	t, ok, err := actions_model.CreateTaskForRunner(ctx, runner)
	if err != nil {
		return nil, false, fmt.Errorf("CreateTaskForRunner: %w", err)
	}
	if !ok {
		return nil, false, nil
	}

	task := &runnerv1.Task{
		Id:              t.ID,
		WorkflowPayload: t.Job.WorkflowPayload,
		Context:         generateTaskContext(t),
		Secrets:         getSecretsOfTask(ctx, t),
	}
	return task, true, nil
}

func getSecretsOfTask(ctx context.Context, task *actions_model.BotTask) map[string]string {
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

	if _, ok := secrets["GITHUB_TOKEN"]; !ok {
		secrets["GITHUB_TOKEN"] = task.Token
	}
	if _, ok := secrets["GITEA_TOKEN"]; !ok {
		secrets["GITEA_TOKEN"] = task.Token
	}

	return secrets
}

func generateTaskContext(t *actions_model.BotTask) *structpb.Struct {
	event := map[string]interface{}{}
	_ = json.Unmarshal([]byte(t.Job.Run.EventPayload), &event)

	taskContext, _ := structpb.NewStruct(map[string]interface{}{
		// standard contexts, see https://docs.github.com/en/actions/learn-github-actions/contexts#github-context
		"action":            "",                                                   // string, The name of the action currently running, or the id of a step. GitHub removes special characters, and uses the name __run when the current step runs a script without an id. If you use the same action more than once in the same job, the name will include a suffix with the sequence number with underscore before it. For example, the first script you run will have the name __run, and the second script will be named __run_2. Similarly, the second invocation of actions/checkout will be actionscheckout2.
		"action_path":       "",                                                   // string, The path where an action is located. This property is only supported in composite actions. You can use this path to access files located in the same repository as the action.
		"action_ref":        "",                                                   // string, For a step executing an action, this is the ref of the action being executed. For example, v2.
		"action_repository": "",                                                   // string, For a step executing an action, this is the owner and repository name of the action. For example, actions/checkout.
		"action_status":     "",                                                   // string, For a composite action, the current result of the composite action.
		"actor":             t.Job.Run.TriggerUser.Name,                           // string, The username of the user that triggered the initial workflow run. If the workflow run is a re-run, this value may differ from github.triggering_actor. Any workflow re-runs will use the privileges of github.actor, even if the actor initiating the re-run (github.triggering_actor) has different privileges.
		"api_url":           "",                                                   // string, The URL of the GitHub REST API.
		"base_ref":          "",                                                   // string, The base_ref or target branch of the pull request in a workflow run. This property is only available when the event that triggers a workflow run is either pull_request or pull_request_target.
		"env":               "",                                                   // string, Path on the runner to the file that sets environment variables from workflow commands. This file is unique to the current step and is a different file for each step in a job. For more information, see "Workflow commands for GitHub Actions."
		"event":             event,                                                // object, The full event webhook payload. You can access individual properties of the event using this context. This object is identical to the webhook payload of the event that triggered the workflow run, and is different for each event. The webhooks for each GitHub Actions event is linked in "Events that trigger workflows." For example, for a workflow run triggered by the push event, this object contains the contents of the push webhook payload.
		"event_name":        t.Job.Run.Event.Event(),                              // string, The name of the event that triggered the workflow run.
		"event_path":        "",                                                   // string, The path to the file on the runner that contains the full event webhook payload.
		"graphql_url":       "",                                                   // string, The URL of the GitHub GraphQL API.
		"head_ref":          "",                                                   // string, The head_ref or source branch of the pull request in a workflow run. This property is only available when the event that triggers a workflow run is either pull_request or pull_request_target.
		"job":               fmt.Sprint(t.JobID),                                  // string, The job_id of the current job.
		"ref":               t.Job.Run.Ref,                                        // string, The fully-formed ref of the branch or tag that triggered the workflow run. For workflows triggered by push, this is the branch or tag ref that was pushed. For workflows triggered by pull_request, this is the pull request merge branch. For workflows triggered by release, this is the release tag created. For other triggers, this is the branch or tag ref that triggered the workflow run. This is only set if a branch or tag is available for the event type. The ref given is fully-formed, meaning that for branches the format is refs/heads/<branch_name>, for pull requests it is refs/pull/<pr_number>/merge, and for tags it is refs/tags/<tag_name>. For example, refs/heads/feature-branch-1.
		"ref_name":          t.Job.Run.Ref,                                        // string, The short ref name of the branch or tag that triggered the workflow run. This value matches the branch or tag name shown on GitHub. For example, feature-branch-1.
		"ref_protected":     false,                                                // boolean, true if branch protections are configured for the ref that triggered the workflow run.
		"ref_type":          "",                                                   // string, The type of ref that triggered the workflow run. Valid values are branch or tag.
		"path":              "",                                                   // string, Path on the runner to the file that sets system PATH variables from workflow commands. This file is unique to the current step and is a different file for each step in a job. For more information, see "Workflow commands for GitHub Actions."
		"repository":        t.Job.Run.Repo.OwnerName + "/" + t.Job.Run.Repo.Name, // string, The owner and repository name. For example, Codertocat/Hello-World.
		"repository_owner":  t.Job.Run.Repo.OwnerName,                             // string, The repository owner's name. For example, Codertocat.
		"repositoryUrl":     t.Job.Run.Repo.HTMLURL(),                             // string, The Git URL to the repository. For example, git://github.com/codertocat/hello-world.git.
		"retention_days":    "",                                                   // string, The number of days that workflow run logs and artifacts are kept.
		"run_id":            fmt.Sprint(t.Job.RunID),                              // string, A unique number for each workflow run within a repository. This number does not change if you re-run the workflow run.
		"run_number":        fmt.Sprint(t.Job.Run.Index),                          // string, A unique number for each run of a particular workflow in a repository. This number begins at 1 for the workflow's first run, and increments with each new run. This number does not change if you re-run the workflow run.
		"run_attempt":       fmt.Sprint(t.Job.Attempt),                            // string, A unique number for each attempt of a particular workflow run in a repository. This number begins at 1 for the workflow run's first attempt, and increments with each re-run.
		"secret_source":     "Actions",                                            // string, The source of a secret used in a workflow. Possible values are None, Actions, Dependabot, or Codespaces.
		"server_url":        setting.AppURL,                                       // string, The URL of the GitHub server. For example: https://github.com.
		"sha":               t.Job.Run.CommitSHA,                                  // string, The commit SHA that triggered the workflow. The value of this commit SHA depends on the event that triggered the workflow. For more information, see "Events that trigger workflows." For example, ffac537e6cbbf934b08745a378932722df287a53.
		"token":             t.Token,                                              // string, A token to authenticate on behalf of the GitHub App installed on your repository. This is functionally equivalent to the GITHUB_TOKEN secret. For more information, see "Automatic token authentication."
		"triggering_actor":  "",                                                   // string, The username of the user that initiated the workflow run. If the workflow run is a re-run, this value may differ from github.actor. Any workflow re-runs will use the privileges of github.actor, even if the actor initiating the re-run (github.triggering_actor) has different privileges.
		"workflow":          t.Job.Run.WorkflowID,                                 // string, The name of the workflow. If the workflow file doesn't specify a name, the value of this property is the full path of the workflow file in the repository.
		"workspace":         "",                                                   // string, The default working directory on the runner for steps, and the default location of your repository when using the checkout action.

		// additional contexts
		"gitea_default_bots_url": setting.Bots.DefaultBotsURL,
	})

	return taskContext
}
