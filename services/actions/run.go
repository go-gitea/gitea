// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/actions/jobparser"
	"code.gitea.io/gitea/modules/util"

	act_model "github.com/nektos/act/pkg/model"
	"go.yaml.in/yaml/v4"
)

// PrepareRunAndInsert prepares a run and inserts it into the database
// It parses the workflow content, evaluates concurrency if needed, and inserts the run and its jobs into the database.
// The title will be cut off at 255 characters if it's longer than 255 characters.
func PrepareRunAndInsert(ctx context.Context, content []byte, run *actions_model.ActionRun, inputsWithDefaults map[string]any) error {
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

		for i, v := range jobs {
			id, job := v.Job()
			needs := job.Needs()
			if err := v.SetJob(id, job.EraseNeeds()); err != nil {
				return err
			}
			payload, _ := v.Marshal()

			shouldBlockJob := runAttempt.Status == actions_model.StatusBlocked || len(needs) > 0 || run.NeedApproval

			job.Name = util.EllipsisDisplayString(job.Name, 255)
			runJob := &actions_model.ActionRunJob{
				RunID:             run.ID,
				RunAttemptID:      runAttempt.ID,
				RepoID:            run.RepoID,
				OwnerID:           run.OwnerID,
				CommitSHA:         run.CommitSHA,
				IsForkPullRequest: run.IsForkPullRequest,
				Name:              job.Name,
				Attempt:           runAttempt.Attempt,
				WorkflowPayload:   payload,
				JobID:             id,
				AttemptJobID:      int64(i + 1),
				Needs:             needs,
				RunsOn:            job.RunsOn(),
				Status:            util.Iif(shouldBlockJob, actions_model.StatusBlocked, actions_model.StatusWaiting),
			}
			// Parse workflow/job permissions (no clamping here)
			if perms := ExtractJobPermissionsFromWorkflow(v, job); perms != nil {
				runJob.TokenPermissions = perms
			}

			// check job concurrency
			if job.RawConcurrency != nil {
				rawConcurrency, err := yaml.Marshal(job.RawConcurrency)
				if err != nil {
					return fmt.Errorf("marshal raw concurrency: %w", err)
				}
				runJob.RawConcurrency = string(rawConcurrency)

				// do not evaluate job concurrency when it requires `needs`, the jobs with `needs` will be evaluated later by job emitter
				if len(needs) == 0 {
					err = EvaluateJobConcurrencyFillModel(ctx, run, runAttempt, runJob, vars, inputs)
					if err != nil {
						return fmt.Errorf("evaluate job concurrency: %w", err)
					}
				}

				// If a job needs other jobs ("needs" is not empty), its status is set to StatusBlocked at the entry of the loop
				// No need to check job concurrency for a blocked job (it will be checked by job emitter later)
				if runJob.Status == actions_model.StatusWaiting {
					var jobsToCancel []*actions_model.ActionRunJob
					runJob.Status, jobsToCancel, err = PrepareToStartJobWithConcurrency(ctx, runJob)
					if err != nil {
						return fmt.Errorf("prepare to start job with concurrency: %w", err)
					}
					cancelledConcurrencyJobs = append(cancelledConcurrencyJobs, jobsToCancel...)
				}
			}

			hasWaitingJobs = hasWaitingJobs || runJob.Status == actions_model.StatusWaiting
			if err := db.Insert(ctx, runJob); err != nil {
				return err
			}

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

	return nil
}
