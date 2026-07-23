// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"gitea.com/gitea/runner/act/exprparser"
	"gitea.com/gitea/runner/act/model"
	"go.yaml.in/yaml/v4"
)

// NewInterpeter returns an interpeter used in the server,
// need github, needs, strategy, matrix, inputs context only,
// see https://docs.github.com/en/actions/learn-github-actions/contexts#context-availability
func NewInterpeter(
	jobID string,
	job *model.Job,
	matrix map[string]any,
	gitCtx *model.GithubContext,
	results map[string]*JobResult,
	vars map[string]string,
	inputs map[string]any,
) exprparser.Interpreter {
	strategy := make(map[string]any)
	if job.Strategy != nil {
		strategy["fail-fast"] = job.Strategy.FailFast
		strategy["max-parallel"] = job.Strategy.MaxParallel
	}

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

	jobs := run.Workflow.Jobs
	jobNeeds := run.Job().Needs()

	using := map[string]exprparser.Needs{}
	for _, need := range jobNeeds {
		if v, ok := jobs[need]; ok {
			using[need] = exprparser.Needs{
				Outputs: v.Outputs,
				Result:  v.Result,
			}
		}
	}

	ee := &exprparser.EvaluationEnvironment{
		Github: gitCtx,
		Env:    nil, // no need
		// Job must be non-nil because cancelled() dereferences Job.Status unconditionally.
		// See: https://gitea.com/gitea/runner/src/commit/ad967330a8788c9b8ab723abbc1a86d53c3bc5e6/act/exprparser/functions.go#L299
		// TODO: The empty JobContext.Status is right for now because Gitea never checks `if` condition when the workflow run is cancelled.
		// This is an implementation gap in Gitea Actions. When a workflow run is cancelled, Gitea should check the job's `if` condition,
		// and if the condition is met (e.g. `if: ${{ cancelled() }}` ), the job should be executed rather than cancelled.
		Job:      &model.JobContext{},
		Steps:    nil, // no need
		Runner:   nil, // no need
		Secrets:  nil, // no need
		Strategy: strategy,
		Matrix:   matrix,
		Needs:    using,
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
