// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"

	act_model "gitea.dev/actionslib/pkg/model"
	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"

	"go.yaml.in/yaml/v4"
)

// PrepareRunAndInsert prepares a run and inserts it into the database
// It parses the workflow content, evaluates concurrency if needed, and inserts the run and its jobs into the database.
// The title will be cut off at 255 characters if it's longer than 255 characters.
func PrepareRunAndInsert(ctx context.Context, content []byte, run *actions_model.ActionRun, inputsWithDefaults map[string]any) error {
	if run.WorkflowRepoID == 0 {
		return fmt.Errorf("WorkflowRepoID must be set before insert (repo %d, workflow %q)", run.RepoID, run.WorkflowID)
	}

	if err := run.LoadAttributes(ctx); err != nil {
		return fmt.Errorf("LoadAttributes: %w", err)
	}

	vars, err := actions_model.GetVariablesOfRun(ctx, run)
	if err != nil {
		return fmt.Errorf("GetVariablesOfRun: %w", err)
	}

	wfRawConcurrency, err := jobparser.ReadWorkflowRawConcurrency(content)
	if err != nil {
		return fmt.Errorf("ReadWorkflowRawConcurrency: %w", err)
	}

	if err = InsertRun(ctx, run, content, vars, inputsWithDefaults, wfRawConcurrency); err != nil {
		return fmt.Errorf("InsertRun: %w", err)
	}

	// Load the newly inserted jobs with all fields from database (the job models in InsertRun are partial, so load again)
	allJobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{RunID: run.ID})
	if err != nil {
		return fmt.Errorf("FindRunJob: %w", err)
	}

	CreateCommitStatusForRunJobs(ctx, run, allJobs...)

	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, allJobs)

	return nil
}

// InsertRun inserts a run
// The title will be cut off at 255 characters if it's longer than 255 characters.
func InsertRun(ctx context.Context, run *actions_model.ActionRun, content []byte, vars map[string]string, inputs map[string]any, wfRawConcurrency *act_model.RawConcurrency) error {
	var cancelledConcurrencyJobs []*actions_model.ActionRunJob
	var needPostCommitEmit bool
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		index, err := db.GetNextResourceIndex(ctx, "action_run_index", run.RepoID)
		if err != nil {
			return err
		}
		run.Index = index
		run.Title = util.EllipsisDisplayString(run.Title, 255)
		run.Status = actions_model.StatusWaiting

		if wfRawConcurrency != nil {
			rawConcurrency, err := yaml.Marshal(wfRawConcurrency)
			if err != nil {
				return fmt.Errorf("marshal raw concurrency: %w", err)
			}
			run.RawConcurrency = string(rawConcurrency)
		}

		// Insert before parsing jobs or evaluating workflow-level concurrency
		// so that run.ID is populated. Expressions referencing github.run_id —
		// in run-name, job names, runs-on, or a workflow-level concurrency
		// group like `${{ github.head_ref || github.run_id }}` — would otherwise
		// interpolate to an empty string.
		if err := db.Insert(ctx, run); err != nil {
			return err
		}

		runAttempt := &actions_model.ActionRunAttempt{
			RepoID:        run.RepoID,
			RunID:         run.ID,
			Attempt:       1,
			TriggerUserID: run.TriggerUserID,
			Status:        actions_model.StatusWaiting,
		}

		if wfRawConcurrency != nil {
			if err := EvaluateRunConcurrencyFillModel(ctx, run, runAttempt, wfRawConcurrency, vars, inputs); err != nil {
				return fmt.Errorf("EvaluateRunConcurrencyFillModel: %w", err)
			}
			// check run (workflow-level) concurrency
			var jobsToCancel []*actions_model.ActionRunJob
			runAttempt.Status, jobsToCancel, err = PrepareToStartRunWithConcurrency(ctx, runAttempt)
			if err != nil {
				return err
			}
			cancelledConcurrencyJobs = append(cancelledConcurrencyJobs, jobsToCancel...)
		}

		if err := db.Insert(ctx, runAttempt); err != nil {
			return err
		}
		run.LatestAttemptID = runAttempt.ID

		giteaCtx := GenerateGiteaContext(ctx, run, runAttempt, nil)
		jobs, err := jobparser.Parse(content, jobparser.WithVars(vars), jobparser.WithGitContext(giteaCtx.ToGitHubContext()), jobparser.WithInputs(inputs))
		if err != nil {
			return fmt.Errorf("parse workflow: %w", err)
		}
		titleChanged := len(jobs) > 0 && jobs[0].RunName != ""
		if titleChanged {
			run.Title = util.EllipsisDisplayString(jobs[0].RunName, 255)
		}

		cols := []string{"latest_attempt_id"}
		if titleChanged {
			cols = append(cols, "title")
		}
		if err := actions_model.UpdateRun(ctx, run, cols...); err != nil {
			return err
		}

		runJobs := make([]*actions_model.ActionRunJob, 0, len(jobs))
		var hasWaitingJobs bool
		slots := maxParallelSlots{}

		for _, v := range jobs {
			runJob, jobsToCancel, jobNeedsPostCommitEmit, err := insertRunJob(ctx, run, runAttempt, v, vars, inputs, slots)
			if err != nil {
				return err
			}
			cancelledConcurrencyJobs = append(cancelledConcurrencyJobs, jobsToCancel...)
			needPostCommitEmit = needPostCommitEmit || jobNeedsPostCommitEmit

			// A reusable caller is never dispatched to a runner, so it must not drive the task-version bump.
			hasWaitingJobs = hasWaitingJobs || (runJob.Status == actions_model.StatusWaiting && !runJob.IsReusableCaller)
			runJobs = append(runJobs, runJob)
		}

		runAttempt.Status = actions_model.AggregateJobStatus(runJobs)
		if err := actions_model.UpdateRunAttempt(ctx, runAttempt, "status"); err != nil {
			return err
		}

		// if there is a job in the waiting status, increase tasks version.
		if hasWaitingJobs {
			if err := actions_model.IncreaseTaskVersion(ctx, run.OwnerID, run.RepoID); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return err
	}

	NotifyWorkflowJobsAndRunsStatusUpdate(ctx, cancelledConcurrencyJobs)
	EmitJobsIfReadyByJobs(cancelledConcurrencyJobs)

	// Post-commit kick: let the job emitter resolve jobs if needed
	if needPostCommitEmit {
		if err := EmitJobsIfReadyByRun(run.ID); err != nil {
			log.Error("emit run %d after InsertRun: %v", run.ID, err)
		}
	}

	return nil
}

// insertRunJob builds a single run job from a parsed workflow job, decides a ready
// no-needs job's `if:`, evaluates its job-level concurrency, inserts it, and — for a
// ready no-needs reusable caller — inline-expands it. It returns the inserted job, any
// jobs cancelled by job concurrency, and whether a post-commit emitter pass is needed
// to resolve the dependents of a skipped job or a caller.
func insertRunJob(ctx context.Context, run *actions_model.ActionRun, runAttempt *actions_model.ActionRunAttempt, workflowJob *jobparser.SingleWorkflow, vars map[string]string, inputs map[string]any, slots maxParallelSlots) (*actions_model.ActionRunJob, []*actions_model.ActionRunJob, bool, error) {
	id, job := workflowJob.Job()
	needs := job.Needs()
	isMatrixDeferred := jobparser.HasDeferredMatrix(job)
	if err := workflowJob.SetJob(id, job.EraseNeeds()); err != nil {
		return nil, nil, false, err
	}
	payload, _ := workflowJob.Marshal()

	isReusableWorkflowCaller := job.Uses != ""
	shouldBlockJob := runAttempt.Status == actions_model.StatusBlocked || len(needs) > 0 || run.NeedApproval

	attemptJobID, err := actions_model.GetNextAttemptJobID(ctx, run.ID)
	if err != nil {
		return nil, nil, false, fmt.Errorf("alloc attempt_job_id: %w", err)
	}

	job.Name = job.DisplayName()
	runJob := &actions_model.ActionRunJob{
		RunID:                   run.ID,
		RunAttemptID:            runAttempt.ID,
		RepoID:                  run.RepoID,
		OwnerID:                 run.OwnerID,
		CommitSHA:               run.CommitSHA,
		IsForkPullRequest:       run.IsForkPullRequest,
		Name:                    job.Name,
		Attempt:                 runAttempt.Attempt,
		WorkflowPayload:         payload,
		JobID:                   id,
		AttemptJobID:            attemptJobID,
		Needs:                   needs,
		RunsOn:                  job.RunsOn(),
		Status:                  util.Iif(shouldBlockJob, actions_model.StatusBlocked, actions_model.StatusWaiting),
		WorkflowSourceRepoID:    run.WorkflowRepoID,
		WorkflowSourceCommitSHA: run.WorkflowCommitSHA,
		ContinueOnError:         job.GetContinueOnError(),
		IsMatrixDeferred:        isMatrixDeferred,
		MaxParallel:             parseMaxParallel(id, job.Strategy.MaxParallelString),
	}
	if isMatrixDeferred {
		// Expansion overwrites WorkflowPayload; keep the raw payload so a rerun can re-derive the matrix.
		runJob.DeferredMatrixPayload = payload
	}
	// Parse workflow/job permissions (no clamping here)
	if perms := ExtractJobPermissionsFromWorkflow(workflowJob, job); perms != nil {
		runJob.TokenPermissions = perms
	}

	if isReusableWorkflowCaller {
		runJob.IsReusableCaller = true
		runJob.CallUses = job.Uses
	}

	// Decide `if:` before concurrency and max-parallel, so a skipped job neither cancels its group peers nor takes a slot.
	// Jobs with `needs` decide it in the job emitter.
	var invalidIfErr error
	if runJob.Status == actions_model.StatusWaiting {
		shouldStart, err := resolveJobIf(ctx, run, runAttempt, runJob, vars, true)
		if errors.Is(err, util.ErrInvalidArgument) {
			invalidIfErr = err
		} else if err != nil {
			return nil, nil, false, fmt.Errorf("evaluate job if: %w", err)
		}
		if !shouldStart {
			runJob.Status = actions_model.StatusSkipped
		}
	}

	var cancelledConcurrencyJobs []*actions_model.ActionRunJob
	// check job concurrency
	if job.RawConcurrency != nil {
		rawConcurrency, err := yaml.Marshal(job.RawConcurrency)
		if err != nil {
			return nil, nil, false, fmt.Errorf("marshal raw concurrency: %w", err)
		}
		runJob.RawConcurrency = string(rawConcurrency)

		// do not evaluate job concurrency when it requires `needs` (the job emitter evaluates it later) or when the job is skipped
		if len(needs) == 0 && runJob.Status != actions_model.StatusSkipped {
			if err := EvaluateJobConcurrencyFillModel(ctx, run, runAttempt, runJob, vars, inputs); err != nil {
				return nil, nil, false, fmt.Errorf("evaluate job concurrency: %w", err)
			}
		}

		// If a job needs other jobs ("needs" is not empty), its status is set to StatusBlocked at the entry of the loop
		// No need to check job concurrency for a blocked job (it will be checked by job emitter later)
		// A slot-starved job skips the check too: it will not start, so it must not cancel its group peers.
		if runJob.Status == actions_model.StatusWaiting && slots.available(runJob) {
			var jobsToCancel []*actions_model.ActionRunJob
			runJob.Status, jobsToCancel, err = PrepareToStartJobWithConcurrency(ctx, runJob)
			if err != nil {
				return nil, nil, false, fmt.Errorf("prepare to start job with concurrency: %w", err)
			}
			cancelledConcurrencyJobs = append(cancelledConcurrencyJobs, jobsToCancel...)
		}
	}

	applyMaxParallel(runJob, slots)

	if err := db.Insert(ctx, runJob); err != nil {
		return nil, nil, false, err
	}
	if invalidIfErr != nil {
		if err := upsertJobErrorSummary(ctx, runJob, "if", invalidIfErr); err != nil {
			return nil, nil, false, err
		}
	}

	// expand reusable caller
	var needPostCommitEmit bool
	if isReusableWorkflowCaller && runJob.Status == actions_model.StatusWaiting {
		if err := expandInlineReusableCaller(ctx, run, runAttempt, runJob, vars); err != nil {
			return nil, nil, false, err
		}
		// an expanded caller needs a resolver pass to resolve its children jobs
		needPostCommitEmit = true
	}
	// a job skipped by its `if:` needs a resolver pass to propagate its state to its dependents
	needPostCommitEmit = needPostCommitEmit || runJob.Status == actions_model.StatusSkipped

	return runJob, cancelledConcurrencyJobs, needPostCommitEmit, nil
}

// expandInlineReusableCaller expands a ready caller into its child jobs and refreshes its status from them.
func expandInlineReusableCaller(ctx context.Context, run *actions_model.ActionRun, runAttempt *actions_model.ActionRunAttempt, caller *actions_model.ActionRunJob, vars map[string]string) error {
	if err := expandReusableWorkflowCaller(ctx, run, runAttempt, caller, vars); err != nil {
		return fmt.Errorf("inline trigger caller %d ready: %w", caller.ID, err)
	}
	// refresh the caller status
	if err := actions_model.RefreshReusableCallerStatus(ctx, caller); err != nil {
		return fmt.Errorf("refresh caller %d status: %w", caller.ID, err)
	}
	return nil
}
