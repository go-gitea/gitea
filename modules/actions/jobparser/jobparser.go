// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"gitea.com/gitea/runner/act/exprparser"
	"gitea.com/gitea/runner/act/model"
	"github.com/rhysd/actionlint"
	"go.yaml.in/yaml/v4"
)

// HasDeferredMatrix reports whether the job's matrix can only be expanded once its needs finish:
// it reads the needs context and the job has needs to resolve that context against.
// Parse emits such a job as a single placeholder rather than one job per combination, so every
// caller that persists a job must agree with Parse on this condition.
func HasDeferredMatrix(job *Job) bool {
	return len(job.Needs()) > 0 && rawMatrixReadsNeeds(&job.Strategy.RawMatrix)
}

func rawMatrixReadsNeeds(node *yaml.Node) bool {
	if node.Kind == yaml.ScalarNode {
		return expressionReadsNeeds(node.Value)
	}
	return slices.ContainsFunc(node.Content, rawMatrixReadsNeeds)
}

// ParseRawSingleWorkflow decodes a SingleWorkflow payload into the workflow and its single job
// without expanding `strategy.matrix`.
//
// A deferred-matrix placeholder's payload still carries the raw, unevaluated matrix, which Parse
// would try to expand: depending on the matrix's shape that yields several workflows (a static
// vector crossed with the unevaluated expression) or an error (an `include`/`exclude` that is still
// a scalar), neither of which describes the one job the payload stands for.
func ParseRawSingleWorkflow(payload []byte) (*SingleWorkflow, *Job, error) {
	swf := &SingleWorkflow{}
	if err := yaml.Unmarshal(payload, swf); err != nil {
		return nil, nil, fmt.Errorf("unmarshal single workflow: %w", err)
	}
	id, job := swf.Job()
	if job == nil {
		return nil, nil, errors.New("payload contains no job")
	}
	if job.Name == "" {
		job.Name = id // Parse defaults it the same way, and callers use it as the job's display name
	}
	return swf, job, nil
}

// expressionReadsNeeds reports whether value holds a ${{ }} expression reading the needs context.
// Every other context (github, vars, inputs, ...) is already available while planning, so deferring
// those too would replace their combinations with one placeholder and change the commit status
// contexts the run publishes, which a repository's required checks are configured against.
func expressionReadsNeeds(value string) bool {
	return expressionReadsContext(value, "needs")
}

// ExpressionReadsMatrix reports whether a job's `if:` reads the matrix context.
// A deferred-matrix placeholder has no combination yet, so such an expression cannot be decided.
func ExpressionReadsMatrix(ifValue string) bool {
	return expressionReadsContext(asIfExpression(ifValue), "matrix")
}

// ExpressionIgnoresNeedResults reports whether a job's `if:` calls always(), failure() or cancelled(),
// the status functions that run a job whatever its needs did rather than under the implicit success().
// Keep in sync with act's exprparser, which owns the same list for the evaluation itself.
func ExpressionIgnoresNeedResults(ifValue string) bool {
	return expressionsMatch(asIfExpression(ifValue), func(node actionlint.ExprNode) bool {
		call, ok := node.(*actionlint.FuncCallNode)
		return ok && slices.Contains([]string{"always", "failure", "cancelled"}, strings.ToLower(call.Callee))
	})
}

// asIfExpression wraps an `if:` that omits the `${{ }}`, which GitHub evaluates as one expression anyway.
// `if:` is the only field with that exception: every other value is interpolated, so a bare matrix or
// `runs-on` is a literal there and must not be parsed as an expression.
func asIfExpression(ifValue string) string {
	if ifValue == "" || strings.Contains(ifValue, "${{") {
		return ifValue
	}
	return "${{ " + ifValue + " }}"
}

// expressionReadsContext reports whether value holds a ${{ }} expression reading the named context.
func expressionReadsContext(value, contextName string) bool {
	return expressionsMatch(value, func(node actionlint.ExprNode) bool {
		variable, ok := node.(*actionlint.VariableNode)
		return ok && strings.EqualFold(variable.Name, contextName)
	})
}

// expressionsMatch reports whether any ${{ }} expression in value holds a node the predicate accepts.
func expressionsMatch(value string, match func(node actionlint.ExprNode) bool) bool {
	for rest := value; ; {
		_, after, found := strings.Cut(rest, "${{")
		if !found {
			return false
		}
		rest = after
		// The lexer ends the expression at its closing `}}`, so it can be handed the whole remainder.
		expr, err := actionlint.NewExprParser().Parse(actionlint.NewExprLexer(rest))
		if err != nil {
			return true // unparseable here, let the expansion report it against the real values
		}
		matched := false
		actionlint.VisitExprNode(expr, func(node, _ actionlint.ExprNode, entering bool) {
			if entering && match(node) {
				matched = true
			}
		})
		if matched {
			return true
		}
	}
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
		results[id] = &JobResult{
			Needs:   job.Needs(),
			Result:  pc.jobResults[id],
			Outputs: nil, // resolved at expansion time, not at plan time
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

		var combos []*Job
		if HasDeferredMatrix(job) {
			// The matrix reads values that do not exist yet (a needs output), so emit a single
			// placeholder keeping it raw. Re-parsing that placeholder's payload yields it again,
			// and the server expands it once the needs finish.
			placeholder := job.Clone()
			if placeholder.Name == "" {
				placeholder.Name = id
			}
			combos = []*Job{placeholder}
		} else {
			matricxes, err := getMatrixes(originJob)
			if err != nil {
				return nil, fmt.Errorf("getMatrixes: %w", err)
			}
			if combos, err = buildMatrixCombos(id, job, matricxes, originJob, pc.gitContext, results, pc.vars, pc.inputs); err != nil {
				return nil, err
			}
		}
		for _, combo := range combos {
			swf := workflow.CloneHeader()
			if err := swf.SetJob(id, combo); err != nil {
				return nil, fmt.Errorf("SetJob: %w", err)
			}
			ret = append(ret, swf)
		}
	}
	return ret, nil
}

// CloneHeader returns a copy of w with its workflow-global fields but no jobs.
func (w *SingleWorkflow) CloneHeader() *SingleWorkflow {
	return &SingleWorkflow{
		Name:           w.Name,
		RawOn:          w.RawOn,
		Env:            w.Env,
		Defaults:       w.Defaults,
		RawPermissions: w.RawPermissions,
		RunName:        w.RunName,
	}
}

// ExpandMatrixWithNeeds returns one Job per combination of job's matrix, resolved against the
// completed needs in results. As for EvaluateConcurrency, results must also describe jobID itself,
// which is where NewInterpeter reads the job's own needs from.
// maxCombinations caps how many combinations may be built: the values come from a needs output at
// runtime, so the cap has to be enforced before one Job is materialized per combination.
func ExpandMatrixWithNeeds(jobID string, job *Job, gitCtx *model.GithubContext, results map[string]*JobResult, vars map[string]string, inputs map[string]any, maxCombinations int) ([]*Job, error) {
	actJob := &model.Job{Strategy: &model.Strategy{
		FailFastString:    job.Strategy.FailFastString,
		MaxParallelString: job.Strategy.MaxParallelString,
		RawMatrix:         job.Strategy.RawMatrix,
	}}

	// Resolve fromJson(needs.*.outputs.*) and friends into concrete matrix values.
	if err := NewExpressionEvaluator(NewInterpeter(jobID, actJob, nil, gitCtx, results, vars, inputs)).
		EvaluateYamlNode(&actJob.Strategy.RawMatrix); err != nil {
		return nil, fmt.Errorf("evaluate matrix: %w", err)
	}
	matrixes, err := getMatrixes(actJob)
	if err != nil {
		return nil, fmt.Errorf("getMatrixes: %w", err)
	}
	// act collapses a matrix that yields no combination (an empty vector or include, everything
	// excluded, a whole-matrix expression that is not a mapping) into one empty combination, which
	// would run the job once unparameterized. GitHub rejects such a matrix, so reject it too.
	if len(matrixes) == 1 && len(matrixes[0]) == 0 {
		return nil, errors.New("matrix must define at least one vector")
	}
	if len(matrixes) > maxCombinations {
		return nil, fmt.Errorf("matrix expands to %d combinations, exceeding the limit of %d", len(matrixes), maxCombinations)
	}
	return buildMatrixCombos(jobID, job, matrixes, actJob, gitCtx, results, vars, inputs)
}

// matrixesOf is this package's only entry to act's GetMatrixes, so that every caller is covered by
// the filter check below. A deferred placeholder is the first thing carrying a raw matrix this far,
// and the emitter reads its `if:` before expanding it.
// TODO: drop the check once gitea.com/gitea/runner validates the shape itself.
func matrixesOf(job *model.Job) ([]map[string]any, error) {
	if err := validateMatrixFilters(job); err != nil {
		return nil, err
	}
	matrixes, err := job.GetMatrixes()
	if err != nil {
		return nil, fmt.Errorf("GetMatrixes: %w", err)
	}
	return matrixes, nil
}

// validateMatrixFilters rejects an `include`/`exclude` that is not a list of mappings. act asserts
// that shape without checking, so anything else panics there; an unevaluated ${{ }} expression, which
// is still a scalar, is the usual way to reach it.
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
			if entry.Kind == yaml.AliasNode {
				entry = entry.Alias
			}
			if entry.Kind != yaml.MappingNode {
				return fmt.Errorf("matrix %s must be a list of mappings", name)
			}
		}
	}
	return nil
}

// buildMatrixCombos builds one Job per matrix combination from src, baking the combination into the
// strategy and interpolating the name, runs-on and continue-on-error with it.
func buildMatrixCombos(jobID string, src *Job, matrixes []map[string]any, actJob *model.Job, gitCtx *model.GithubContext, results map[string]*JobResult, vars map[string]string, inputs map[string]any) ([]*Job, error) {
	srcRunsOn := src.RunsOn()
	combos := make([]*Job, 0, len(matrixes))
	for _, matrix := range matrixes {
		combo := src.Clone()
		if combo.Name == "" {
			combo.Name = jobID
		}
		combo.Strategy.RawMatrix = encodeMatrix(matrix)
		evaluator := NewExpressionEvaluator(NewInterpeter(jobID, actJob, matrix, gitCtx, results, vars, inputs))
		combo.Name = nameWithMatrix(combo.Name, matrix, evaluator)
		runsOn := slices.Clone(srcRunsOn)
		for i := range runsOn {
			runsOn[i] = evaluator.Interpolate(runsOn[i])
		}
		combo.RawRunsOn = encodeRunsOn(runsOn)
		if err := evaluator.EvaluateYamlNode(&combo.RawContinueOnError); err != nil {
			return nil, fmt.Errorf("evaluate continue-on-error for job %q: %w", jobID, err)
		}
		combos = append(combos, combo)
	}
	return combos, nil
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
	ret, err := matrixesOf(job)
	if err != nil {
		return nil, err
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
