// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"gitea.com/gitea/runner/act/exprparser"
	"gitea.com/gitea/runner/act/model"
	"github.com/rhysd/actionlint"
	"go.yaml.in/yaml/v4"
)

func Parse(content []byte, options ...ParseOption) ([]*SingleWorkflow, error) {
	// The workflow is split into one document per job below, which would strand an alias whose
	// anchor lands in another one.
	doc, err := resolveYamlAliases(content)
	if err != nil {
		return nil, fmt.Errorf("resolve aliases: %w", err)
	}

	origin, err := readWorkflowDoc(doc)
	if err != nil {
		return nil, fmt.Errorf("read workflow: %w", err)
	}

	workflow := &SingleWorkflow{}
	if err := decodeYamlDoc(doc, workflow); err != nil {
		return nil, fmt.Errorf("decode workflow: %w", err)
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
	if workflow.RunName, err = evaluator.interpolate(workflow.RunName); err != nil {
		return nil, fmt.Errorf("interpolate run-name: %w", err)
	}

	for i, id := range ids {
		job := jobs[i]
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
			if job.Name, err = nameWithMatrix(job.Name, matrix, evaluator); err != nil {
				return nil, fmt.Errorf("interpolate name for job %q: %w", id, err)
			}
			runsOn := origin.GetJob(id).RunsOn()
			for i, v := range runsOn {
				if runsOn[i], err = evaluator.interpolate(v); err != nil {
					return nil, fmt.Errorf("interpolate runs-on for job %q: %w", id, err)
				}
			}
			job.RawRunsOn = encodeRunsOn(runsOn)
			if err := evaluator.EvaluateYamlNode(&job.RawContinueOnError); err != nil {
				return nil, fmt.Errorf("evaluate continue-on-error for job %q: %w", id, err)
			}
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

func getMatrixes(job *model.Job) ([]map[string]any, error) {
	if err := validateMatrixFilters(job); err != nil {
		return nil, err
	}
	ret, err := job.GetMatrixes()
	if err != nil {
		return nil, fmt.Errorf("GetMatrixes: %w", err)
	}
	sort.Slice(ret, func(i, j int) bool {
		return matrixName(ret[i]) < matrixName(ret[j])
	})
	return ret, nil
}

// validateMatrixFilters rejects an `include`/`exclude` that is not a list of mappings, so that the
// usual way to get there, an unevaluated ${{ }} expression that is still a scalar, is named as such
// instead of panicking inside the expansion.
func validateMatrixFilters(job *model.Job) error {
	if job.Strategy == nil || job.Strategy.RawMatrix.Kind != yaml.MappingNode {
		return nil
	}
	content := job.Strategy.RawMatrix.Content
	for i := 0; i+1 < len(content); i += 2 {
		name, value := content[i].Value, content[i+1]
		if name != "include" && name != "exclude" {
			continue
		}
		entries := []*yaml.Node{value}
		if value.Kind == yaml.SequenceNode {
			entries = value.Content
		}
		for _, entry := range entries {
			if entry.Kind != yaml.MappingNode {
				return fmt.Errorf("matrix %s must be a list of mappings", name)
			}
		}
	}
	return nil
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

func nameWithMatrix(name string, m map[string]any, evaluator *ExpressionEvaluator) (string, error) {
	if len(m) == 0 {
		return name, nil
	}

	if !strings.Contains(name, "${{") || !strings.Contains(name, "}}") {
		return name + " " + matrixName(m), nil
	}

	return evaluator.interpolate(name)
}

// expressionCallsFunction reports whether any ${{ }} expression in value calls one of the functions.
func expressionCallsFunction(value string, names ...string) bool {
	parts, err := splitSubExpressions(value)
	if err != nil {
		return true // unparseable here, let the expansion report it against the real values
	}
	for _, part := range parts {
		if !part.isExpr {
			continue
		}
		// The lexer needs the closing `}}` that the scanner strips.
		expr, err := actionlint.NewExprParser().Parse(actionlint.NewExprLexer(part.text + "}}"))
		if err != nil {
			return true // unparseable here, let the expansion report it against the real values
		}
		found := false
		actionlint.VisitExprNode(expr, func(node, _ actionlint.ExprNode, entering bool) {
			call, ok := node.(*actionlint.FuncCallNode)
			if entering && ok && slices.Contains(names, strings.ToLower(call.Callee)) {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
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
