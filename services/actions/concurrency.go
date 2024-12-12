// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"bytes"
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/util"

	"github.com/nektos/act/pkg/jobparser"
	act_model "github.com/nektos/act/pkg/model"
)

func PrepareConcurrencyForRunAndJobs(ctx context.Context, wfContent []byte, run *actions_model.ActionRun, jobs []*jobparser.SingleWorkflow) ([]*actions_model.ActionRunJob, error) {
	vars, err := actions_model.GetVariablesOfRun(ctx, run)
	if err != nil {
		return nil, fmt.Errorf("get run %d variables: %w", run.ID, err)
	}

	// check workflow concurrency
	wfRawConcurrency, err := jobparser.ReadWorkflowRawConcurrency(wfContent)
	if err != nil {
		return nil, fmt.Errorf("read workflow raw concurrency: %w", err)
	}
	if wfRawConcurrency != nil {
		wfGitCtx := jobparser.ToGitContext(GenerateGitContext(run, nil))
		wfConcurrencyGroup, wfConcurrencyCancel := jobparser.EvaluateWorkflowConcurrency(wfRawConcurrency, wfGitCtx, vars)
		if len(wfConcurrencyGroup) > 0 {
			run.ConcurrencyGroup = wfConcurrencyGroup
			run.ConcurrencyCancel = wfConcurrencyCancel
		}
	}

	runJobs := make([]*actions_model.ActionRunJob, 0, len(jobs))
	for _, v := range jobs {
		id, job := v.Job()
		needs := job.Needs()
		if err := v.SetJob(id, job.EraseNeeds()); err != nil {
			return nil, err
		}
		payload, _ := v.Marshal()

		status := actions_model.StatusWaiting
		if len(needs) > 0 || run.NeedApproval {
			status = actions_model.StatusBlocked
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
			if runJob.Status != actions_model.StatusBlocked && len(needs) == 0 {
				var err error
				runJob.ConcurrencyGroup, runJob.ConcurrencyCancel, err = evaluateJobConcurrency(ctx, runJob, vars, map[string]*jobparser.JobResult{})
				if err != nil {
					return nil, fmt.Errorf("evaluate job concurrency: %w", err)
				}
			}
		}

		runJobs = append(runJobs, runJob)
	}

	return runJobs, nil
}

func evaluateJobConcurrency(ctx context.Context, actionRunJob *actions_model.ActionRunJob, vars map[string]string, jobResults map[string]*jobparser.JobResult) (string, bool, error) {
	if err := actionRunJob.LoadRun(ctx); err != nil {
		return "", false, err
	}
	run := actionRunJob.Run

	rawConcurrency := &act_model.RawConcurrency{
		Group:            actionRunJob.RawConcurrencyGroup,
		CancelInProgress: actionRunJob.RawConcurrencyCancel,
	}

	gitCtx := jobparser.ToGitContext(GenerateGitContext(run, actionRunJob))

	actWorkflow, err := act_model.ReadWorkflow(bytes.NewReader(actionRunJob.WorkflowPayload))
	if err != nil {
		return "", false, fmt.Errorf("read workflow: %w", err)
	}
	actJob := actWorkflow.GetJob(actionRunJob.JobID)

	concurrencyGroup, concurrencyCancel := jobparser.EvaluateJobConcurrency(rawConcurrency, actJob, gitCtx, vars, jobResults)

	return concurrencyGroup, concurrencyCancel, nil
}
