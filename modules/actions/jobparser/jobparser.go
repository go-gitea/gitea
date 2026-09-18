// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"gitea.dev/actionslib/pkg/expreval"
	"gitea.dev/actionslib/pkg/exprparser"
	"gitea.dev/actionslib/pkg/model"

	"go.yaml.in/yaml/v4"
)

// HasDeferredMatrix reports whether the job's matrix can only be expanded once its needs finish:
// it or another field of the strategy reads the needs context and the job has needs to resolve that context against.
// Parse emits such a job as a single placeholder rather than one job per combination, so every
// caller that persists a job must agree with Parse on this condition.
func HasDeferredMatrix(job *Job) bool {
	return len(job.Needs()) > 0 && (nodeMatches(&job.Strategy.RawMatrix, expressionReadsNeeds) || expressionReadsNeeds(job.Strategy.RawExpression.Value) ||
		job.Strategy.RawMatrix.Kind != 0 && (expressionReadsNeeds(job.Strategy.MaxParallelString) || expressionReadsNeeds(job.Strategy.FailFastString)))
}

func nodeMatches(node *yaml.Node, match func(string) bool) bool {
	if node.Kind == yaml.ScalarNode {
		return match(node.Value)
	}
	return slices.ContainsFunc(node.Content, func(child *yaml.Node) bool { return nodeMatches(child, match) })
}

func hasExpression(value string) bool {
	return strings.Contains(value, "${{")
}

// ParseRawSingleWorkflow decodes a stored SingleWorkflow payload into the workflow and its single job as stored, without expanding or evaluating it again.
func ParseRawSingleWorkflow(payload []byte) (*SingleWorkflow, *Job, error) {
	swf := &SingleWorkflow{}
	if err := decodeResolved(payload, swf); err != nil {
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
	return expreval.ReadsContext(value, "needs")
}

// ExpressionReadsMatrix reports whether a job's `if:` reads the matrix context.
// A deferred-matrix placeholder has no combination yet, so such an expression cannot be decided.
func ExpressionReadsMatrix(ifValue string) bool {
	return expreval.ReadsContext(asIfExpression(ifValue), "matrix")
}

// ExpressionIgnoresNeedResults reports whether a job's `if:` calls always(), failure() or cancelled(),
// the status functions that run a job whatever its needs did rather than under the implicit success().
// Keep in sync with act's exprparser, which owns the same list for the evaluation itself.
func ExpressionIgnoresNeedResults(ifValue string) bool {
	return expreval.CallsFunction(asIfExpression(ifValue), "always", "failure", "cancelled")
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
			Outputs: nil, // resolved at expansion time, not at plan time
		}
	}

	var ret []*SingleWorkflow
	ids, jobs, err := workflow.jobs()
	if err != nil {
		return nil, fmt.Errorf("invalid jobs: %w", err)
	}

	evaluator := expreval.New(exprparser.NewInterpeter(&exprparser.EvaluationEnvironment{Github: pc.gitContext, Vars: pc.vars, Inputs: pc.inputs}, exprparser.Config{}).Evaluate)
	if workflow.RunName, err = evaluator.Interpolate(workflow.RunName); err != nil {
		return nil, fmt.Errorf("interpolate run-name: %w", err)
	}

	for i, id := range ids {
		job := jobs[i]
		originJob := origin.GetJob(id)

		if originJob == nil {
			return nil, fmt.Errorf("job %s not found in origin workflow", id)
		}

		var combos []*Job
		if HasDeferredMatrix(job) || pc.gitContext == nil && (job.Strategy.RawExpression.Kind != 0 || nodeMatches(&job.Strategy.RawMatrix, hasExpression)) {
			// The strategy reads values that do not exist yet (a needs output, or any context without
			// a git context), so emit a single placeholder keeping it raw. Re-parsing that placeholder's
			// payload yields it again, and the server expands it once the needs finish.
			placeholder := job.Clone()
			if placeholder.Name == "" {
				placeholder.Name = id
			}
			combos = []*Job{placeholder}
		} else {
			if pc.gitContext != nil { // callers without one, like commit status, only read the literal workflow
				if err := job.Strategy.resolve(evaluator); err != nil {
					return nil, fmt.Errorf("job %q: %w", id, err)
				}
				originJob.Strategy = job.Strategy.actStrategy()
			}
			matrixes, err := originJob.GetMatrixes()
			if err != nil {
				return nil, fmt.Errorf("getMatrixes: %w", err)
			}
			if combos, err = buildMatrixCombos(id, job, matrixes, pc.gitContext, results, pc.vars, pc.inputs); err != nil {
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
	job = job.Clone()
	if err := job.Strategy.resolve(expreval.New(NewInterpeter(jobID, nil, nil, gitCtx, results, vars, inputs).Evaluate)); err != nil {
		return nil, err
	}
	matrixes, err := (&model.Job{Strategy: job.Strategy.actStrategy()}).GetMatrixes()
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
	return buildMatrixCombos(jobID, job, matrixes, gitCtx, results, vars, inputs)
}

// resolve evaluates the strategy and escapes fail-fast and max-parallel for the runner, leaving unevaluable ones to its defaults.
func (s *Strategy) resolve(evaluator expreval.Evaluator) error {
	if s.RawExpression.Kind != 0 {
		if err := model.DecodeEvaluated("strategy", s.RawExpression, evaluator.EvaluateYamlNode, s); err != nil {
			return err
		}
		if s.RawExpression.Kind != 0 {
			return errors.New("strategy is not a map of strategy keys to values")
		}
	} else {
		for _, value := range []*string{&s.FailFastString, &s.MaxParallelString} {
			if evaluated, err := evaluator.Interpolate(*value); err == nil {
				*value = evaluated
			}
		}
		if err := evaluator.EvaluateYamlNode(&s.RawMatrix); err != nil {
			return fmt.Errorf("evaluate matrix: %w", err)
		}
	}
	s.FailFastString, s.MaxParallelString = escapeExpressions(s.FailFastString), escapeExpressions(s.MaxParallelString)
	return nil
}

const escapedExpression = "${{ '$' }}{{"

// escapeExpressions returns a template interpolating to value, since GitHub never evaluates the result of an evaluation again.
func escapeExpressions(value string) string {
	return strings.ReplaceAll(value, "${{", escapedExpression)
}

func unescapeExpressions(value string) string {
	return strings.ReplaceAll(value, escapedExpression, "${{")
}

func replaceScalars(node *yaml.Node, replace func(string) string) {
	node.Value = replace(node.Value)
	for _, child := range node.Content {
		replaceScalars(child, replace)
	}
}

// buildMatrixCombos builds one Job per matrix combination from src, baking the combination into the
// strategy and interpolating the name, runs-on and continue-on-error with it.
func buildMatrixCombos(jobID string, src *Job, matrixes []map[string]any, gitCtx *model.GithubContext, results map[string]*JobResult, vars map[string]string, inputs map[string]any) ([]*Job, error) {
	srcRunsOn := model.RunsOnFromNode(src.RawRunsOn)
	order, names := make([]int, len(matrixes)), make([]string, len(matrixes))
	for index, matrix := range matrixes {
		order[index], names[index] = index, matrixName(matrix)
	}
	slices.SortStableFunc(order, func(a, b int) int { return strings.Compare(names[a], names[b]) })
	combos := make([]*Job, 0, len(matrixes))
	var err error
	for _, index := range order {
		matrix := matrixes[index]
		combo := src.Clone()
		if combo.Name == "" {
			combo.Name = jobID
		}
		combo.Strategy.RawMatrix = encodeMatrix(matrix)
		replaceScalars(&combo.Strategy.RawMatrix, escapeExpressions)
		if len(matrix) > 0 {
			combo.Strategy.JobIndex, combo.Strategy.JobTotal = index, len(matrixes)
		}
		evaluator := expreval.New(NewInterpeter(jobID, &combo.Strategy, matrix, gitCtx, results, vars, inputs).Evaluate)
		if combo.Name, err = nameWithMatrix(combo.Name, matrix, evaluator); err != nil {
			return nil, fmt.Errorf("interpolate name for job %q: %w", jobID, err)
		}
		runsOn := slices.Clone(srcRunsOn)
		for i := range runsOn {
			if runsOn[i], err = evaluator.Interpolate(runsOn[i]); err != nil {
				return nil, fmt.Errorf("interpolate runs-on for job %q: %w", jobID, err)
			}
			runsOn[i] = escapeExpressions(runsOn[i])
		}
		combo.RawRunsOn = model.RunsOnNode(runsOn, "")
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

func nameWithMatrix(name string, m map[string]any, evaluator expreval.Evaluator) (string, error) {
	if len(m) == 0 {
		return name, nil
	}

	if !strings.Contains(name, "${{") || !strings.Contains(name, "}}") {
		return escapeExpressions(name + " " + matrixName(m)), nil
	}

	name, err := evaluator.Interpolate(name)
	return escapeExpressions(name), err
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
