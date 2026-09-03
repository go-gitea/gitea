// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	runnerv1 "gitea.dev/actionslib/runner/v1"
	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	secret_model "gitea.dev/models/secret"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"google.golang.org/protobuf/types/known/structpb"
)

var (
	taskPickSem     chan struct{}
	taskPickSemOnce sync.Once
)

func taskPickLimiter() chan struct{} {
	taskPickSemOnce.Do(func() {
		taskPickSem = make(chan struct{}, max(1, setting.Actions.MaxConcurrentTaskPicks))
	})
	return taskPickSem
}

// TryPickTask attempts to assign a task to the runner, bounding the number of
// concurrent assignment transactions to avoid a thundering herd when many
// runners poll at once. When the concurrency limit is reached it returns
// throttled=true without touching the DB, so the caller can let the runner
// retry on its next poll instead of advancing its tasks version.
func TryPickTask(ctx context.Context, runner *actions_model.ActionRunner) (task *runnerv1.Task, ok, throttled bool, err error) {
	sem := taskPickLimiter()
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	default:
		return nil, false, true, nil
	}
	task, ok, err = PickTask(ctx, runner)
	return task, ok, false, err
}

// releaseTaskForRunnerCleanup releases a claimed task using a fresh, bounded context. The request context
// is typically already canceled when we reach the release paths below, and a DB transaction on a canceled
// context fails immediately, which would strand the claimed job in running state.
func releaseTaskForRunnerCleanup(t *actions_model.ActionTask) {
	ctx, cancel := context.WithTimeout(graceful.GetManager().ShutdownContext(), 10*time.Second)
	defer cancel()
	if relErr := actions_model.ReleaseTaskForRunner(ctx, t); relErr != nil {
		log.Error("ReleaseTaskForRunner [task_id: %d]: %v", t.ID, relErr)
	}
}

func PickTask(ctx context.Context, runner *actions_model.ActionRunner) (*runnerv1.Task, bool, error) {
	var (
		task       *runnerv1.Task
		job        *actions_model.ActionRunJob
		actionTask *actions_model.ActionTask
	)

	if runner.IsDisabled {
		return nil, false, nil
	}

	if runner.Ephemeral {
		var task actions_model.ActionTask
		has, err := db.GetEngine(ctx).Where("runner_id = ?", runner.ID).Get(&task)
		// Let the runner retry the request, do not allow to proceed
		if err != nil {
			return nil, false, err
		}
		if has {
			if task.Status.In(actions_model.StatusWaiting, actions_model.StatusRunning, actions_model.StatusBlocked, actions_model.StatusCancelling) {
				return nil, false, nil
			}
			// task has been finished, remove it
			_, err = db.DeleteByID[actions_model.ActionRunner](ctx, runner.ID)
			if err != nil {
				return nil, false, err
			}
			return nil, false, errors.New("runner has been removed")
		}
	}

	t, ok, err := actions_model.CreateTaskForRunner(ctx, runner)
	if err != nil {
		return nil, false, fmt.Errorf("CreateTaskForRunner: %w", err)
	}
	if !ok {
		return nil, false, nil
	}

	// resolved once here: the secrets and variables of the task are scoped by the same environment
	env, allowed, err := ResolveJobEnvironment(ctx, t.Job)
	if err != nil {
		releaseTaskForRunnerCleanup(t)
		return nil, false, fmt.Errorf("resolve environment of job %d: %w", t.Job.ID, err)
	}
	if !allowed {
		if err := denyJobByEnvironment(ctx, t, env); err != nil {
			releaseTaskForRunnerCleanup(t)
			return nil, false, err
		}
		return nil, false, nil
	}

	task, job, err = buildRunnerTask(ctx, t, env)
	if err != nil {
		// The job was already claimed but assembling its payload failed; release the
		// claim so the job returns to the waiting queue instead of being stranded in
		// running state with no runner ever executing it.
		releaseTaskForRunnerCleanup(t)
		return nil, false, err
	}
	actionTask = t

	CreateCommitStatusForRunJobs(ctx, job.Run, job)
	NotifyWorkflowJobStatusUpdateWithTask(ctx, job, actionTask)
	// job.Run is loaded inside the transaction before UpdateRunJob sets run.Started,
	// so Started is zero only on the very first pick-up of that run.
	if job.Run.Started.IsZero() {
		NotifyWorkflowRunStatusUpdateWithReload(ctx, job.RepoID, job.RunID)
	}

	// The job is claimed and its payload assembled, but if the request context was cancelled meanwhile, response can no longer reach the runner.
	// Release the claim so another runner can pick the job up.
	if err := ctx.Err(); err != nil {
		releaseTaskForRunnerCleanup(t)
		return nil, false, err
	}

	return task, true, nil
}

// denyJobByEnvironment fails an already-claimed job that may not deploy: its ref is refused by the
// environment's branch policy, or (env == nil) no environment exists under the name the workflow gave.
// Running it with the environment's secrets withheld would fail later and less legibly.
func denyJobByEnvironment(ctx context.Context, t *actions_model.ActionTask, env *actions_model.ActionEnvironment) error {
	reason := fmt.Sprintf("Environment `%s` does not exist.", t.Job.EnvironmentName)
	if env != nil {
		reason = fmt.Sprintf("Branch is not allowed to deploy to `%s` due to environment protection rules.", env.Name)
	}
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := actions_model.StopTask(ctx, t.ID, actions_model.StatusFailure); err != nil {
			return err
		}
		return actions_model.UpsertActionRunJobSummary(ctx, t.RepoID, t.Job.RunID, t.Job.RunAttemptID, t.Job.ID, 0,
			actions_model.JobSummaryContentTypeMarkdown, []byte(reason))
	}); err != nil {
		return fmt.Errorf("fail job %d on environment policy: %w", t.Job.ID, err)
	}

	log.Info("Job %d denied: %s", t.Job.ID, reason)
	// StopTask wrote the failure through a fresh model, so the claimed job still reads as running
	job, err := actions_model.GetRunJobByRepoAndID(ctx, t.RepoID, t.JobID)
	if err != nil {
		return fmt.Errorf("reload denied job %d: %w", t.JobID, err)
	}
	job.Run = t.Job.Run
	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, []*actions_model.ActionRunJob{job})
	EmitJobsIfReadyByJobs([]*actions_model.ActionRunJob{job})
	return nil
}

// buildRunnerTask assembles the runner-facing task payload for an already-claimed task, whose
// attributes the claim loaded. All operations are read-only; on error the caller releases the claim.
func buildRunnerTask(ctx context.Context, t *actions_model.ActionTask, env *actions_model.ActionEnvironment) (*runnerv1.Task, *actions_model.ActionRunJob, error) {
	job := t.Job

	secrets, err := secret_model.GetSecretsOfTask(ctx, t, env)
	if err != nil {
		return nil, nil, fmt.Errorf("GetSecretsOfTask: %w", err)
	}

	vars, err := actions_model.GetVariablesOfJob(ctx, t.Job, env)
	if err != nil {
		return nil, nil, fmt.Errorf("GetVariablesOfJob: %w", err)
	}

	needs, err := findTaskNeeds(ctx, job)
	if err != nil {
		return nil, nil, fmt.Errorf("findTaskNeeds: %w", err)
	}

	taskContext, err := generateTaskContext(ctx, t)
	if err != nil {
		return nil, nil, fmt.Errorf("generateTaskContext: %w", err)
	}

	return &runnerv1.Task{
		Id:              t.ID,
		WorkflowPayload: t.Job.WorkflowPayload,
		Context:         taskContext,
		Secrets:         secrets,
		Vars:            vars,
		Needs:           needs,
	}, job, nil
}

func generateTaskContext(ctx context.Context, t *actions_model.ActionTask) (*structpb.Struct, error) {
	giteaRuntimeToken, err := CreateAuthorizationToken(t.ID, t.Job.RunID, t.JobID)
	if err != nil {
		return nil, err
	}

	gitCtx := GenerateGiteaContext(ctx, t.Job.Run, nil, t.Job)
	gitCtx["token"] = t.Token
	gitCtx["gitea_runtime_token"] = giteaRuntimeToken

	return structpb.NewStruct(gitCtx)
}

func findTaskNeeds(ctx context.Context, taskJob *actions_model.ActionRunJob) (map[string]*runnerv1.TaskNeed, error) {
	taskNeeds, err := FindTaskNeeds(ctx, taskJob)
	if err != nil {
		return nil, err
	}
	ret := make(map[string]*runnerv1.TaskNeed, len(taskNeeds))
	for jobID, taskNeed := range taskNeeds {
		ret[jobID] = &runnerv1.TaskNeed{
			Outputs: taskNeed.Outputs,
			Result:  runnerv1.Result(taskNeed.Result),
		}
	}
	return ret, nil
}
