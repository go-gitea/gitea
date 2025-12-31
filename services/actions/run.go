// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"
	notify_service "code.gitea.io/gitea/services/notify"

	"github.com/nektos/act/pkg/jobparser"
	"gopkg.in/yaml.v3"
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

	if wfRawConcurrency != nil {
		err = EvaluateRunConcurrencyFillModel(ctx, run, wfRawConcurrency, vars)
		if err != nil {
			return fmt.Errorf("EvaluateRunConcurrencyFillModel: %w", err)
		}
	}

	giteaCtx := GenerateGiteaContext(run, nil)

	jobs, err := jobparser.Parse(content, jobparser.WithVars(vars), jobparser.WithGitContext(giteaCtx.ToGitHubContext()), jobparser.WithInputs(inputsWithDefaults))
	if err != nil {
		return fmt.Errorf("parse workflow: %w", err)
	}

	if len(jobs) > 0 && jobs[0].RunName != "" {
		run.Title = jobs[0].RunName
	}

	if err = InsertRun(ctx, run, jobs, vars); err != nil {
		return fmt.Errorf("InsertRun: %w", err)
	}

	// Load the newly inserted jobs with all fields from database (the job models in InsertRun are partial, so load again)
	allJobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{RunID: run.ID})
	if err != nil {
		return fmt.Errorf("FindRunJob: %w", err)
	}

	CreateCommitStatusForRunJobs(ctx, run, allJobs...)

	notify_service.WorkflowRunStatusUpdate(ctx, run.Repo, run.TriggerUser, run)
	for _, job := range allJobs {
		notify_service.WorkflowJobStatusUpdate(ctx, run.Repo, run.TriggerUser, job, nil)
	}

	return nil
}

// InsertRun inserts a run
// The title will be cut off at 255 characters if it's longer than 255 characters.
func InsertRun(ctx context.Context, run *actions_model.ActionRun, jobs []*jobparser.SingleWorkflow, vars map[string]string) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		index, err := db.GetNextResourceIndex(ctx, "action_run_index", run.RepoID)
		if err != nil {
			return err
		}
		run.Index = index
		run.Title = util.EllipsisDisplayString(run.Title, 255)

		// check run (workflow-level) concurrency
		run.Status, err = PrepareToStartRunWithConcurrency(ctx, run)
		if err != nil {
			return err
		}

		if err := db.Insert(ctx, run); err != nil {
			return err
		}

		// GitVM: emit receipt for CI run start
		if setting.GitVM.Enabled {
			emitCIRunStartReceipt(ctx, run)
		}

		if err := run.LoadRepo(ctx); err != nil {
			return err
		}

		if err := actions_model.UpdateRepoRunsNumbers(ctx, run.Repo); err != nil {
			return err
		}

		runJobs := make([]*actions_model.ActionRunJob, 0, len(jobs))
		var hasWaitingJobs bool
		for _, v := range jobs {
			id, job := v.Job()
			needs := job.Needs()
			if err := v.SetJob(id, job.EraseNeeds()); err != nil {
				return err
			}
			payload, _ := v.Marshal()

			shouldBlockJob := len(needs) > 0 || run.NeedApproval || run.Status == actions_model.StatusBlocked

			job.Name = util.EllipsisDisplayString(job.Name, 255)
			runJob := &actions_model.ActionRunJob{
				RunID:             run.ID,
				RepoID:            run.RepoID,
				OwnerID:           run.OwnerID,
				CommitSHA:         run.CommitSHA,
				IsForkPullRequest: run.IsForkPullRequest,
				Name:              job.Name,
				WorkflowPayload:   payload,
				JobID:             id,
				Needs:             needs,
				RunsOn:            job.RunsOn(),
				Status:            util.Iif(shouldBlockJob, actions_model.StatusBlocked, actions_model.StatusWaiting),
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
					err = EvaluateJobConcurrencyFillModel(ctx, run, runJob, vars)
					if err != nil {
						return fmt.Errorf("evaluate job concurrency: %w", err)
					}
				}

				// If a job needs other jobs ("needs" is not empty), its status is set to StatusBlocked at the entry of the loop
				// No need to check job concurrency for a blocked job (it will be checked by job emitter later)
				if runJob.Status == actions_model.StatusWaiting {
					runJob.Status, err = PrepareToStartJobWithConcurrency(ctx, runJob)
					if err != nil {
						return fmt.Errorf("prepare to start job with concurrency: %w", err)
					}
				}
			}

			hasWaitingJobs = hasWaitingJobs || runJob.Status == actions_model.StatusWaiting
			if err := db.Insert(ctx, runJob); err != nil {
				return err
			}

			runJobs = append(runJobs, runJob)
		}

		run.Status = actions_model.AggregateJobStatus(runJobs)
		if err := actions_model.UpdateRun(ctx, run, "status"); err != nil {
			return err
		}

		// if there is a job in the waiting status, increase tasks version.
		if hasWaitingJobs {
			if err := actions_model.IncreaseTaskVersion(ctx, run.OwnerID, run.RepoID); err != nil {
				return err
			}
		}

		return nil
	})
}
