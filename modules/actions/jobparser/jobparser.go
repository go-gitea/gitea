// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gitea.com/gitea/runner/act/exprparser"
	"gitea.com/gitea/runner/act/model"
	"go.yaml.in/yaml/v4"
)

func Parse(content []byte, options ...ParseOption) ([]*SingleWorkflow, error) {
	origin, err := model.ReadWorkflow(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("model.ReadWorkflow: %w", err)
	}

	workflow := &SingleWorkflow{}
	if err := yaml.Unmarshal(content, workflow); err != nil {
		return nil, fmt.Errorf("yaml.Unmarshal: %w", err)
	}

	pc := &parseContext{}
	for _, o := range options {
		o(pc)
	}
	results := map[string]*JobResult{}
	for id, job := range origin.Jobs {
		if job == nil {
			return nil, fmt.Errorf("needed job not found: %q", id)
		}
		results[id] = &JobResult{
			Needs:   job.Needs(),
			Result:  pc.jobResults[id],
			Outputs: nil, // not supported yet
		}
	}

	var ret []*SingleWorkflow
	ids, jobs, err := workflow.jobs()
	if err != nil {
		return nil, fmt.Errorf("invalid jobs: %w", err)
	}

	evaluator := NewExpressionEvaluator(exprparser.NewInterpeter(&exprparser.EvaluationEnvironment{Github: pc.gitContext, Vars: pc.vars, Inputs: pc.inputs}, exprparser.Config{}))
	workflow.RunName = evaluator.Interpolate(workflow.RunName)

	for i, id := range ids {
		job := jobs[i]

		// Deferred matrix expansion: if strategy.matrix references upstream
		// `needs.<id>.outputs.<key>`, the value isn't known at submission time.
		// Emit a single placeholder SingleWorkflow carrying the un-evaluated
		// matrix. The job emitter re-runs expansion once the needs jobs finish.
		if MatrixDependsOnNeedsOutputs(job.Strategy.RawMatrix) {
			swf := &SingleWorkflow{
				Name:           workflow.Name,
				RawOn:          workflow.RawOn,
				Env:            workflow.Env,
				Defaults:       workflow.Defaults,
				RawPermissions: workflow.RawPermissions,
				RunName:        workflow.RunName,
			}
			if err := swf.SetJob(id, job.Clone()); err != nil {
				return nil, fmt.Errorf("SetJob: %w", err)
			}
			ret = append(ret, swf)
			continue
		}

		matricxes, err := getMatrixes(origin.GetJob(id))
		if err != nil {
			return nil, fmt.Errorf("getMatrixes: %w", err)
		}
		for _, matrix := range matricxes {
			job := job.Clone()
			if job.Name == "" {
				job.Name = id
			}
			job.Strategy.RawMatrix = encodeMatrix(matrix)
			evaluator := NewExpressionEvaluator(NewInterpeter(id, origin.GetJob(id), matrix, pc.gitContext, results, pc.vars, pc.inputs))
			job.Name = nameWithMatrix(job.Name, matrix, evaluator)
			runsOn := origin.GetJob(id).RunsOn()
			for i, v := range runsOn {
				runsOn[i] = evaluator.Interpolate(v)
			}
			job.RawRunsOn = encodeRunsOn(runsOn)
			swf := &SingleWorkflow{
				Name:           workflow.Name,
				RawOn:          workflow.RawOn,
				Env:            workflow.Env,
				Defaults:       workflow.Defaults,
				RawPermissions: workflow.RawPermissions,
				RunName:        workflow.RunName,
			}
			if err := swf.SetJob(id, job); err != nil {
				return nil, fmt.Errorf("SetJob: %w", err)
			}
			ret = append(ret, swf)
		}
	}
	return ret, nil
}

func WithGitContext(context *model.GithubContext) ParseOption {
	return func(c *parseContext) {
		c.gitContext = context
	}
}

func WithVars(vars map[string]string) ParseOption {
	return func(c *parseContext) {
		c.vars = vars
	}
}

func WithInputs(inputs map[string]any) ParseOption {
	return func(c *parseContext) {
		c.inputs = inputs
	}
}

type parseContext struct {
	jobResults map[string]string
	gitContext *model.GithubContext
	vars       map[string]string
	inputs     map[string]any
}

type ParseOption func(c *parseContext)

// ExpandDeferredMatrix re-parses a placeholder SingleWorkflow (as emitted
// by Parse when strategy.matrix references upstream job outputs) and expands
// it into N concrete iterations, using the provided upstream needs outputs
// to resolve `${{ ... needs.<id>.outputs.<key> ... }}` references inside the
// matrix YAML.
//
// `content` is the placeholder's serialized SingleWorkflow (the value stored
// in ActionRunJob.WorkflowPayload). `needs` lists the placeholder job's
// upstream job IDs; `outputs` maps each upstream job ID to its merged outputs.
// Pass any GitHub/vars/inputs context via the option helpers.
func ExpandDeferredMatrix(content []byte, needs []string, outputs map[string]map[string]string, options ...ParseOption) ([]*SingleWorkflow, error) {
	workflow := &SingleWorkflow{}
	if err := yaml.Unmarshal(content, workflow); err != nil {
		return nil, fmt.Errorf("yaml.Unmarshal: %w", err)
	}

	pc := &parseContext{}
	for _, o := range options {
		o(pc)
	}

	ids, jobs, err := workflow.jobs()
	if err != nil {
		return nil, fmt.Errorf("invalid jobs: %w", err)
	}
	if len(ids) != 1 {
		return nil, fmt.Errorf("ExpandDeferredMatrix: expected single job, got %d", len(ids))
	}
	id, job := ids[0], jobs[0]

	results := map[string]*JobResult{}
	// NewInterpeter expects the placeholder's own JobID to be present in the
	// run's workflow.Jobs map (it reads run.Job().Needs() to filter the
	// available `needs.*` context). Add a stub entry for the placeholder
	// itself before the upstream needs.
	results[id] = &JobResult{Needs: needs}
	for _, needID := range needs {
		needOutputs := outputs[needID]
		if needOutputs == nil {
			needOutputs = map[string]string{}
		}
		results[needID] = &JobResult{
			Needs:   nil,
			Result:  "success",
			Outputs: needOutputs,
		}
	}

	// Build a model.Job to feed into the interpreter / matrix-expansion helpers.
	// RawNeeds must reflect the placeholder's needs list so the interpreter
	// populates the `needs.*` context for the matrix expression.
	var rawNeeds yaml.Node
	if len(needs) > 0 {
		if err := rawNeeds.Encode(needs); err != nil {
			return nil, fmt.Errorf("encode needs: %w", err)
		}
	}
	actJob := &model.Job{
		Name:      job.Name,
		RawNeeds:  rawNeeds,
		RawRunsOn: job.RawRunsOn,
		Env:       job.Env,
		If:        job.If,
		Strategy: &model.Strategy{
			FailFastString:    job.Strategy.FailFastString,
			MaxParallelString: job.Strategy.MaxParallelString,
			RawMatrix:         job.Strategy.RawMatrix,
		},
	}

	// Resolve ${{ ... }} expressions inside the matrix YAML using the needs
	// outputs context. After this, the matrix node holds concrete values.
	evaluator := NewExpressionEvaluator(NewInterpeter(id, actJob, nil, pc.gitContext, results, pc.vars, pc.inputs))
	if err := evaluator.EvaluateYamlNode(&actJob.Strategy.RawMatrix); err != nil {
		return nil, fmt.Errorf("evaluate matrix YAML: %w", err)
	}
	// Mirror the evaluated matrix back onto the parsed SingleWorkflow's Job
	// so the per-iteration clones below carry the resolved values.
	job.Strategy.RawMatrix = actJob.Strategy.RawMatrix

	matrixes, err := getMatrixes(actJob)
	if err != nil {
		return nil, fmt.Errorf("getMatrixes: %w", err)
	}
	if len(matrixes) == 0 {
		return nil, fmt.Errorf("matrix expansion produced no combinations for job %q", id)
	}

	var ret []*SingleWorkflow
	for _, matrix := range matrixes {
		clonedJob := job.Clone()
		if clonedJob.Name == "" {
			clonedJob.Name = id
		}
		clonedJob.Strategy.RawMatrix = encodeMatrix(matrix)
		iterEval := NewExpressionEvaluator(NewInterpeter(id, actJob, matrix, pc.gitContext, results, pc.vars, pc.inputs))
		clonedJob.Name = nameWithMatrix(clonedJob.Name, matrix, iterEval)
		runsOn := actJob.RunsOn()
		for i, v := range runsOn {
			runsOn[i] = iterEval.Interpolate(v)
		}
		clonedJob.RawRunsOn = encodeRunsOn(runsOn)
		swf := &SingleWorkflow{
			Name:           workflow.Name,
			RawOn:          workflow.RawOn,
			Env:            workflow.Env,
			Defaults:       workflow.Defaults,
			RawPermissions: workflow.RawPermissions,
			RunName:        workflow.RunName,
		}
		if err := swf.SetJob(id, clonedJob); err != nil {
			return nil, fmt.Errorf("SetJob: %w", err)
		}
		ret = append(ret, swf)
	}
	return ret, nil
}

// matrixOutputsRefRegex matches `${{ ... needs.<id>.outputs.<key> ... }}`
// inside a matrix YAML node. Such expressions cannot be evaluated at
// submission time and force deferred matrix expansion.
var matrixOutputsRefRegex = regexp.MustCompile(`\$\{\{[^}]*\bneeds\.[a-zA-Z0-9_-]+\.outputs\.`)

// MatrixDependsOnNeedsOutputs reports whether the given strategy.matrix YAML
// node references `needs.<id>.outputs.<key>` and therefore needs to be
// expanded after the upstream jobs finish.
func MatrixDependsOnNeedsOutputs(node yaml.Node) bool {
	if node.Kind == 0 {
		return false
	}
	buf, err := yaml.Marshal(&node)
	if err != nil {
		return false
	}
	return matrixOutputsRefRegex.Match(buf)
}

func getMatrixes(job *model.Job) ([]map[string]any, error) {
	ret, err := job.GetMatrixes()
	if err != nil {
		return nil, fmt.Errorf("GetMatrixes: %w", err)
	}
	sort.Slice(ret, func(i, j int) bool {
		return matrixName(ret[i]) < matrixName(ret[j])
	})
	return ret, nil
}

func encodeMatrix(matrix map[string]any) yaml.Node {
	if len(matrix) == 0 {
		return yaml.Node{}
	}
	value := map[string][]any{}
	for k, v := range matrix {
		value[k] = []any{v}
	}
	node := yaml.Node{}
	_ = node.Encode(value)
	return node
}

func encodeRunsOn(runsOn []string) yaml.Node {
	node := yaml.Node{}
	if len(runsOn) == 1 {
		_ = node.Encode(runsOn[0])
	} else {
		_ = node.Encode(runsOn)
	}
	return node
}

func nameWithMatrix(name string, m map[string]any, evaluator *ExpressionEvaluator) string {
	if len(m) == 0 {
		return name
	}

	if !strings.Contains(name, "${{") || !strings.Contains(name, "}}") {
		return name + " " + matrixName(m)
	}

	return evaluator.Interpolate(name)
}

func matrixName(m map[string]any) string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	vs := make([]string, 0, len(m))
	for _, v := range ks {
		vs = append(vs, fmt.Sprint(m[v]))
	}

	return fmt.Sprintf("(%s)", strings.Join(vs, ", "))
}
