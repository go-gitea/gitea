// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strings"

	"gitea.dev/modules/log"

	"gitea.com/gitea/runner/act/exprparser"
	"gitea.com/gitea/runner/act/model"
	"go.yaml.in/yaml/v4"
)

// deepCopyYamlNode creates a deep copy of a yaml.Node to prevent mutations
// from affecting the original. This is important because yaml.Node.Content
// is a slice of pointers, and a shallow copy would share the same child nodes.
func deepCopyYamlNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	nodeCopy := *node
	if node.Content != nil {
		nodeCopy.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			nodeCopy.Content[i] = deepCopyYamlNode(child)
		}
	}
	return &nodeCopy
}

// rawMatrixHasExpression reports whether any scalar in the matrix node contains a
// ${{ }} expression, i.e. the matrix must be evaluated rather than used verbatim.
func rawMatrixHasExpression(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.ScalarNode {
		return strings.Contains(node.Value, "${{")
	}
	return slices.ContainsFunc(node.Content, rawMatrixHasExpression)
}

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
		outputs := pc.jobOutputs[id]
		results[id] = &JobResult{
			Needs:   job.Needs(),
			Result:  pc.jobResults[id],
			Outputs: outputs,
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
		originJob := origin.GetJob(id)

		if originJob == nil {
			return nil, fmt.Errorf("job %s not found in origin workflow", id)
		}

		// Clone + pre-evaluate only when the matrix has a ${{ }} expression; static matrices use
		// the origin job as-is.
		evaluatedJob := originJob
		if originJob.Strategy != nil && rawMatrixHasExpression(&originJob.Strategy.RawMatrix) {
			jobCopy := *originJob
			stratCopy := *originJob.Strategy
			stratCopy.RawMatrix = *deepCopyYamlNode(&originJob.Strategy.RawMatrix)
			jobCopy.Strategy = &stratCopy
			evaluatedJob = &jobCopy
			matrixEvaluator := NewExpressionEvaluator(NewInterpeter(id, evaluatedJob, nil, pc.gitContext, results, pc.vars, pc.inputs))
			if err := matrixEvaluator.EvaluateYamlNode(&evaluatedJob.Strategy.RawMatrix); err != nil {
				// Matrix references needs.*.outputs.* (unavailable now). Emit one placeholder
				// keeping the raw strategy; the server expands it once the needs finish.
				if len(originJob.Needs()) > 0 {
					placeholder := job.Clone()
					if placeholder.Name == "" {
						placeholder.Name = id
					}
					swf := newSingleWorkflow(workflow)
					if err := swf.SetJob(id, placeholder); err != nil {
						return nil, fmt.Errorf("SetJob: %w", err)
					}
					ret = append(ret, swf)
					continue
				}
				log.Debug("matrix evaluation for job %s left unresolved (no needs): %v", id, err)
			}
		}

		matricxes, err := getMatrixes(evaluatedJob)
		if err != nil {
			return nil, fmt.Errorf("getMatrixes: %w", err)
		}
		for _, matrix := range matricxes {
			swf := newSingleWorkflow(workflow)
			if err := swf.SetJob(id, expandJobCombo(id, job, matrix, evaluatedJob, pc.gitContext, results, pc.vars, pc.inputs)); err != nil {
				return nil, fmt.Errorf("SetJob: %w", err)
			}
			ret = append(ret, swf)
		}
	}
	return ret, nil
}

// newSingleWorkflow returns a SingleWorkflow carrying w's global fields and no job.
func newSingleWorkflow(w *SingleWorkflow) *SingleWorkflow {
	return &SingleWorkflow{
		Name:           w.Name,
		RawOn:          w.RawOn,
		Env:            w.Env,
		Defaults:       w.Defaults,
		RawPermissions: w.RawPermissions,
		RunName:        w.RunName,
	}
}

// expandJobCombo builds the Job for one matrix combination: it bakes the matrix into the strategy
// and interpolates the name and runs-on. actJob drives the interpreter (it supplies the job's
// needs and strategy contexts) and may differ from src (an act model.Job vs the source *Job).
func expandJobCombo(jobID string, src *Job, matrix map[string]any, actJob *model.Job, gitCtx *model.GithubContext, results map[string]*JobResult, vars map[string]string, inputs map[string]any) *Job {
	combo := src.Clone()
	if combo.Name == "" {
		combo.Name = jobID
	}
	combo.Strategy.RawMatrix = encodeMatrix(matrix)
	evaluator := NewExpressionEvaluator(NewInterpeter(jobID, actJob, matrix, gitCtx, results, vars, inputs))
	combo.Name = nameWithMatrix(combo.Name, matrix, evaluator)
	runsOn := combo.RunsOn()
	for i := range runsOn {
		runsOn[i] = evaluator.Interpolate(runsOn[i])
	}
	combo.RawRunsOn = encodeRunsOn(runsOn)
	return combo
}

// RawMatrixHasExpression reports whether the job's matrix contains a ${{ }} expression.
// With needs present, this marks a matrix whose expansion is deferred until they complete.
func RawMatrixHasExpression(job *Job) bool {
	return rawMatrixHasExpression(&job.Strategy.RawMatrix)
}

// ExpandMatrixWithNeeds expands job's matrix once its needs complete, returning one Job per
// combination. Like EvaluateConcurrency it evaluates against the needs context via NewInterpeter,
// with no workflow re-parse or stub jobs. job must carry its raw matrix and needs (RawNeeds).
func ExpandMatrixWithNeeds(jobID string, job *Job, gitCtx *model.GithubContext, results map[string]*JobResult, vars map[string]string, inputs map[string]any) ([]*Job, error) {
	var rawNeeds yaml.Node
	if err := rawNeeds.Encode(job.Needs()); err != nil {
		return nil, fmt.Errorf("encode needs: %w", err)
	}
	actJob := &model.Job{
		RawNeeds: rawNeeds,
		Strategy: &model.Strategy{
			FailFastString:    job.Strategy.FailFastString,
			MaxParallelString: job.Strategy.MaxParallelString,
			RawMatrix:         *deepCopyYamlNode(&job.Strategy.RawMatrix),
		},
	}

	// Resolve fromJson(needs.*.outputs.*) and friends into concrete matrix values.
	if err := NewExpressionEvaluator(NewInterpeter(jobID, actJob, nil, gitCtx, results, vars, inputs)).
		EvaluateYamlNode(&actJob.Strategy.RawMatrix); err != nil {
		return nil, fmt.Errorf("evaluate matrix: %w", err)
	}
	matrixes, err := getMatrixes(actJob)
	if err != nil {
		return nil, fmt.Errorf("getMatrixes: %w", err)
	}

	expanded := make([]*Job, 0, len(matrixes))
	for _, matrix := range matrixes {
		expanded = append(expanded, expandJobCombo(jobID, job, matrix, actJob, gitCtx, results, vars, inputs))
	}
	return expanded, nil
}

func WithJobResults(results map[string]string) ParseOption {
	return func(c *parseContext) {
		c.jobResults = results
	}
}

func WithJobOutputs(outputs map[string]map[string]string) ParseOption {
	return func(c *parseContext) {
		c.jobOutputs = outputs
	}
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
	jobOutputs map[string]map[string]string
	gitContext *model.GithubContext
	vars       map[string]string
	inputs     map[string]any
}

type ParseOption func(c *parseContext)

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
