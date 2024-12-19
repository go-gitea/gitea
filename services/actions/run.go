// Copyright 2024 The Gitea Authors. All rights reserved.
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
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	index, err := db.GetNextResourceIndex(ctx, "action_run_index", run.RepoID)
	if err != nil {
		return err
	}
	run.Index = index
	run.Title, _ = util.SplitStringAtByteN(run.Title, 255)

	// check workflow concurrency
	if len(run.ConcurrencyGroup) > 0 {
		if run.ConcurrencyCancel {
			if err := actions_model.CancelPreviousJobsWithOpts(ctx, &actions_model.FindRunOptions{
				RepoID:           run.RepoID,
				ConcurrencyGroup: run.ConcurrencyGroup,
				Status: []actions_model.Status{
					actions_model.StatusRunning,
					actions_model.StatusWaiting,
					actions_model.StatusBlocked,
				},
			}); err != nil {
				return err
			}
		} else {
			concurrentRunsNum, err := db.Count[actions_model.ActionRun](ctx, &actions_model.FindRunOptions{
				RepoID:           run.RepoID,
				ConcurrencyGroup: run.ConcurrencyGroup,
				Status:           []actions_model.Status{actions_model.StatusWaiting, actions_model.StatusRunning},
			})
			if err != nil {
				return err
			}
			if concurrentRunsNum > 0 {
				run.Status = actions_model.StatusBlocked
			}
		}
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
		job.Name, _ = util.SplitStringAtByteN(job.Name, 255)
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
		if job.RawConcurrency != nil && len(job.RawConcurrency.Group) > 0 {
			runJob.RawConcurrencyGroup = job.RawConcurrency.Group
			runJob.RawConcurrencyCancel = job.RawConcurrency.CancelInProgress
			// we do not need to evaluate job concurrency if the job is blocked
			// because it will be checked by job emitter
			if runJob.Status != actions_model.StatusBlocked {
				var err error
				runJob.ConcurrencyGroup, runJob.ConcurrencyCancel, err = evaluateJobConcurrency(run, runJob, vars, map[string]*jobparser.JobResult{})
				if err != nil {
					return fmt.Errorf("evaluate job concurrency: %w", err)
				}
				if len(runJob.ConcurrencyGroup) > 0 {
					// check if the job should be blocked by job concurrency
					shouldBlock, err := actions_model.ShouldJobBeBlockedByConcurrentJobs(ctx, runJob)
					if err != nil {
						return err
					}
					if shouldBlock {
						runJob.Status = actions_model.StatusBlocked
					}
				}
			}
		}

		runJobs = append(runJobs, runJob)
	}

	if err := db.Insert(ctx, runJobs); err != nil {
		return err
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

	return committer.Commit()
}
