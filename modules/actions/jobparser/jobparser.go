// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

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

		// Clone the origin job to avoid modifying the shared object
		evaluatedJob := *originJob
		if originJob.Strategy != nil {
			stratCopy := *originJob.Strategy
			// Deep copy the RawMatrix yaml.Node to prevent mutations from affecting the original
			stratCopy.RawMatrix = *deepCopyYamlNode(&originJob.Strategy.RawMatrix)
			evaluatedJob.Strategy = &stratCopy
		}

		// Create an evaluator with access to needs/outputs for matrix evaluation
		matrixEvaluator := NewExpressionEvaluator(NewInterpeter(id, &evaluatedJob, nil, pc.gitContext, results, pc.vars, pc.inputs))

		// Evaluate the matrix before expanding it
		if evaluatedJob.Strategy != nil && evaluatedJob.Strategy.RawMatrix.Kind != 0 {
			if err := matrixEvaluator.EvaluateYamlNode(&evaluatedJob.Strategy.RawMatrix); err != nil {
				return nil, fmt.Errorf("error evaluating matrix for job %s: %w", id, err)
			}
		}

		matricxes, err := getMatrixes(&evaluatedJob)
		if err != nil {
			return nil, fmt.Errorf("getMatrixes: %w", err)
		}
		for _, matrix := range matricxes {
			job := job.Clone()
			if job.Name == "" {
				job.Name = id
			}
			job.Strategy.RawMatrix = encodeMatrix(matrix)
			evaluator := NewExpressionEvaluator(NewInterpeter(id, &evaluatedJob, matrix, pc.gitContext, results, pc.vars, pc.inputs))
			job.Name = nameWithMatrix(job.Name, matrix, evaluator)
			runsOn := evaluatedJob.RunsOn()
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
