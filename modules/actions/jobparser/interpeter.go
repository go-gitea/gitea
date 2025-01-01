package jobparser

import (
	"github.com/nektos/act/pkg/exprparser"
	"github.com/nektos/act/pkg/model"
	"gopkg.in/yaml.v3"
)

// NewInterpeter returns an interpeter used in the server,
// need github, needs, strategy, matrix, inputs context only,
// see https://docs.github.com/en/actions/learn-github-actions/contexts#context-availability
func NewInterpeter(
	jobID string,
	job *model.Job,
	matrix map[string]interface{},
	gitCtx *model.GithubContext,
	results map[string]*JobResult,
	vars map[string]string,
) exprparser.Interpreter {
	strategy := make(map[string]interface{})
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
		Github:   gitCtx,
		Env:      nil, // no need
		Job:      nil, // no need
		Steps:    nil, // no need
		Runner:   nil, // no need
		Secrets:  nil, // no need
		Strategy: strategy,
		Matrix:   matrix,
		Needs:    using,
		Inputs:   nil, // not supported yet
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
