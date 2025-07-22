// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/util"

	"github.com/nektos/act/pkg/jobparser"
)

// InsertRun inserts a run
// The title will be cut off at 255 characters if it's longer than 255 characters.
func InsertRun(ctx context.Context, run *actions_model.ActionRun, jobs []*jobparser.SingleWorkflow) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		index, err := db.GetNextResourceIndex(ctx, "action_run_index", run.RepoID)
		if err != nil {
			return err
		}
		run.Index = index
		run.Title = util.EllipsisDisplayString(run.Title, 255)

		// check run (workflow-level) concurrency
		blockRunByConcurrency, err := actions_model.ShouldBlockRunByConcurrency(ctx, run)
		if err != nil {
			return err
		}
		if blockRunByConcurrency {
			run.Status = actions_model.StatusBlocked
		}
		if err := CancelJobsByRunConcurrency(ctx, run); err != nil {
			return fmt.Errorf("cancel jobs: %w", err)
		}

		if err := db.Insert(ctx, run); err != nil {
			return err
		}

		if run.Repo == nil {
			repo, err := repo_model.GetRepositoryByID(ctx, run.RepoID)
			if err != nil {
				return err
			}
			run.Repo = repo
		}

		if err := actions_model.UpdateRepoRunsNumbers(ctx, run.Repo); err != nil {
			return err
		}

		// query vars for evaluating job concurrency groups
		vars, err := actions_model.GetVariablesOfRun(ctx, run)
		if err != nil {
			return fmt.Errorf("get run %d variables: %w", run.ID, err)
		}

		runJobs := make([]*actions_model.ActionRunJob, 0, len(jobs))
		var hasWaiting bool
		for _, v := range jobs {
			id, job := v.Job()
			needs := job.Needs()
			if err := v.SetJob(id, job.EraseNeeds()); err != nil {
				return err
			}
			payload, _ := v.Marshal()
			status := actions_model.StatusWaiting
			if len(needs) > 0 || run.NeedApproval || run.Status == actions_model.StatusBlocked {
				status = actions_model.StatusBlocked
			} else {
				hasWaiting = true
			}
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
				Status:            status,
			}
			// check job concurrency
			if job.RawConcurrency != nil && job.RawConcurrency.Group != "" {
				runJob.RawConcurrencyGroup = job.RawConcurrency.Group
				runJob.RawConcurrencyCancel = job.RawConcurrency.CancelInProgress
				// do not evaluate job concurrency when it requires `needs`
				if len(needs) == 0 {
					var err error
					runJob.ConcurrencyGroup, runJob.ConcurrencyCancel, err = EvaluateJobConcurrency(ctx, run, runJob, vars, nil)
					if err != nil {
						return fmt.Errorf("evaluate job concurrency: %w", err)
					}
					runJob.IsConcurrencyEvaluated = true
				}
				// do not need to check job concurrency if the job is blocked because it will be checked by job emitter
				if runJob.Status != actions_model.StatusBlocked {
					// check if the job should be blocked by job concurrency
					blockByConcurrency, err := actions_model.ShouldBlockJobByConcurrency(ctx, runJob)
					if err != nil {
						return err
					}
					if blockByConcurrency {
						runJob.Status = actions_model.StatusBlocked
					}
					if err := CancelJobsByJobConcurrency(ctx, runJob); err != nil {
						return fmt.Errorf("cancel jobs: %w", err)
					}
				}
			}

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
		if hasWaiting {
			if err := actions_model.IncreaseTaskVersion(ctx, run.OwnerID, run.RepoID); err != nil {
				return err
			}
		}

		return nil
	})
}
