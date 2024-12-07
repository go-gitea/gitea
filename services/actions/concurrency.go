// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"bytes"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"

	"github.com/nektos/act/pkg/jobparser"
	act_model "github.com/nektos/act/pkg/model"
)

func evaluateJobConcurrency(run *actions_model.ActionRun, actionRunJob *actions_model.ActionRunJob, vars map[string]string, jobResults map[string]*jobparser.JobResult) (string, bool, error) {
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

	if len(jobResults) == 0 {
		jobResults = map[string]*jobparser.JobResult{actionRunJob.JobID: {}}
	}

	concurrencyGroup, concurrencyCancel := jobparser.EvaluateJobConcurrency(rawConcurrency, actionRunJob.JobID, actJob, gitCtx, vars, jobResults)

	return concurrencyGroup, concurrencyCancel, nil
}
