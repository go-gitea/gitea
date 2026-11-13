// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"gitea.dev/actionslib/pkg/exprparser"
	"gitea.dev/actionslib/pkg/model"

	"go.yaml.in/yaml/v4"
)

// NewInterpeter returns an interpeter used in the server,
// need github, needs, strategy, matrix, inputs context only,
// see https://docs.github.com/en/actions/learn-github-actions/contexts#context-availability
func NewInterpeter(
	jobID string,
	strategy *Strategy,
	matrix map[string]any,
	gitCtx *model.GithubContext,
	results map[string]*JobResult,
	vars map[string]string,
	inputs map[string]any,
) exprparser.Interpreter {
	run := &model.Run{
		Workflow: &model.Workflow{
			Jobs: map[string]*model.Job{},
		},
		JobID: jobID,
	}
	for id, result := range results {
		need := yaml.Node{}
		_ = need.Encode(result.Needs)
		run.Workflow.Jobs[id] = &model.Job{
			RawNeeds: need,
			Result:   result.Result,
			Outputs:  result.Outputs,
		}
	}

	ee := &exprparser.EvaluationEnvironment{
		Github: gitCtx,
		Env:    nil, // no need
		// TODO: The empty JobContext.Status is right for now because Gitea never checks `if` condition when the workflow run is cancelled.
		// This is an implementation gap in Gitea Actions. When a workflow run is cancelled, Gitea should check the job's `if` condition,
		// and if the condition is met (e.g. `if: ${{ cancelled() }}` ), the job should be executed rather than cancelled.
		Job:      &model.JobContext{},
		Steps:    nil, // no need
		Runner:   nil, // no need
		Secrets:  nil, // no need
		Strategy: strategy.context(),
		Matrix:   matrix,
		Needs:    exprparser.NeedsContext(run),
		Inputs:   inputs,
		Vars:     vars,
	}

	config := exprparser.Config{
		Run:        run,
		WorkingDir: "", // WorkingDir is used for  the function hashFiles, but it's not needed in the server
		Context:    "job",
	}

	return exprparser.NewInterpeter(ee, config)
}

// JobResult is the minimum requirement of job results for Interpeter
type JobResult struct {
	Needs   []string
	Result  string
	Outputs map[string]string
}
