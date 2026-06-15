// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gitea.dev/modules/container"
	"gitea.dev/modules/util"

	"gitea.com/gitea/runner/act/exprparser"
	"gitea.com/gitea/runner/act/model"
	"go.yaml.in/yaml/v4"
)

// InputType enumerates the allowed types for a workflow_call input.
type InputType string

const (
	InputTypeString  InputType = "string"
	InputTypeBoolean InputType = "boolean"
	InputTypeNumber  InputType = "number"
)

// InputSpec describes a single workflow_call input declaration.
type InputSpec struct {
	Description string    `yaml:"description"`
	Required    bool      `yaml:"required"`
	Default     yaml.Node `yaml:"default"`
	Type        InputType `yaml:"type"`
}

// SecretSpec describes a single workflow_call secret declaration.
type SecretSpec struct {
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

// OutputSpec describes a single workflow_call output declaration.
type OutputSpec struct {
	Description string `yaml:"description"`
	Value       string `yaml:"value"`
}

// WorkflowCallSpec is the parsed "on.workflow_call" schema of a called workflow.
type WorkflowCallSpec struct {
	Inputs  map[string]InputSpec
	Secrets map[string]SecretSpec
	Outputs map[string]OutputSpec
}

// JobOutputs is the per-job-id outputs map used for evaluating workflow_call outputs.
type JobOutputs map[string]map[string]string

// ParseWorkflowCallSpec extracts on.workflow_call.{inputs,secrets,outputs} from a workflow YAML.
// Returns an error if the workflow does not declare on.workflow_call at all.
func ParseWorkflowCallSpec(content []byte) (*WorkflowCallSpec, error) {
	var doc struct {
		On yaml.Node `yaml:"on"`
	}
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("parse workflow yaml: %w", err)
	}

	wcNode, ok := findWorkflowCallNode(&doc.On)
	if !ok {
		return nil, errors.New("workflow does not declare on.workflow_call")
	}

	spec := &WorkflowCallSpec{
		Inputs:  map[string]InputSpec{},
		Secrets: map[string]SecretSpec{},
		Outputs: map[string]OutputSpec{},
	}

	if wcNode == nil || wcNode.Kind != yaml.MappingNode {
		return spec, nil
	}

	for i := 0; i+1 < len(wcNode.Content); i += 2 {
		key := wcNode.Content[i]
		val := wcNode.Content[i+1]
		switch key.Value {
		case "inputs":
			if err := decodeWorkflowCallMapping(val, spec.Inputs); err != nil {
				return nil, fmt.Errorf("parse workflow_call.inputs: %w", err)
			}
		case "secrets":
			if err := decodeWorkflowCallMapping(val, spec.Secrets); err != nil {
				return nil, fmt.Errorf("parse workflow_call.secrets: %w", err)
			}
		case "outputs":
			if err := decodeWorkflowCallMapping(val, spec.Outputs); err != nil {
				return nil, fmt.Errorf("parse workflow_call.outputs: %w", err)
			}
		}
	}

	for name, in := range spec.Inputs {
		if in.Type == "" {
			return nil, fmt.Errorf("workflow_call input %q is missing required field \"type\"", name)
		}
		switch in.Type {
		case InputTypeString, InputTypeBoolean, InputTypeNumber:
		default:
			return nil, fmt.Errorf("workflow_call input %q has unsupported type %q", name, in.Type)
		}
	}

	return spec, nil
}

// findWorkflowCallNode walks the "on:" node and returns the value mapping (or nil) for "workflow_call".
// "ok" is true when the workflow declares workflow_call (even with an empty body).
func findWorkflowCallNode(on *yaml.Node) (val *yaml.Node, ok bool) {
	if on == nil || on.Kind == 0 {
		return nil, false
	}
	switch on.Kind {
	case yaml.ScalarNode:
		return nil, on.Value == "workflow_call"
	case yaml.SequenceNode:
		for _, item := range on.Content {
			if item.Kind == yaml.ScalarNode && item.Value == "workflow_call" {
				return nil, true
			}
		}
		return nil, false
	case yaml.MappingNode:
		for i := 0; i+1 < len(on.Content); i += 2 {
			k := on.Content[i]
			v := on.Content[i+1]
			if k.Value != "workflow_call" {
				continue
			}
			if v.Kind == yaml.MappingNode {
				return v, true
			}
			return nil, true
		}
	}
	return nil, false
}

func decodeWorkflowCallMapping[T any](node *yaml.Node, dst map[string]T) error {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		name := node.Content[i].Value
		var v T
		if err := node.Content[i+1].Decode(&v); err != nil {
			return fmt.Errorf("%q: %w", name, err)
		}
		dst[name] = v
	}
	return nil
}

// EvaluateCallerWith evaluates the caller-side expressions in `job.With` against the provided contexts
func EvaluateCallerWith(
	jobID string,
	job *Job,
	gitCtx map[string]any,
	results map[string]*JobResult,
	vars map[string]string,
	inputs map[string]any,
) (map[string]any, error) {
	actJob := &model.Job{Strategy: &model.Strategy{
		FailFastString:    job.Strategy.FailFastString,
		MaxParallelString: job.Strategy.MaxParallelString,
		RawMatrix:         job.Strategy.RawMatrix,
	}}

	var matrix map[string]any
	matrixes, err := actJob.GetMatrixes()
	if err != nil {
		return nil, fmt.Errorf("get caller %q matrix: %w", jobID, err)
	}
	if len(matrixes) > 0 {
		matrix = matrixes[0]
	}

	evaluator := NewExpressionEvaluator(NewInterpeter(jobID, actJob, matrix, toGitContext(gitCtx), results, vars, inputs))

	out := make(map[string]any, len(job.With))
	for k, raw := range job.With {
		var evaluated any
		switch v := raw.(type) {
		case string:
			node := yaml.Node{}
			if err := node.Encode(v); err != nil {
				return nil, fmt.Errorf("encode caller %q with[%q]: %w", jobID, k, err)
			}
			if err := evaluator.EvaluateYamlNode(&node); err != nil {
				return nil, fmt.Errorf("evaluate caller %q with[%q]: %w", jobID, k, err)
			}
			if err := node.Decode(&evaluated); err != nil {
				return nil, fmt.Errorf("decode caller %q with[%q]: %w", jobID, k, err)
			}
		default:
			evaluated = v
		}
		out[k] = evaluated
	}
	return out, nil
}

// MatchCallerInputsAgainstSpec checks the caller's already-evaluated `with:` values against the callee's declared `on.workflow_call.inputs` schema
func MatchCallerInputsAgainstSpec(spec *WorkflowCallSpec, evaluated map[string]any) (map[string]any, error) {
	resolved := make(map[string]any, len(spec.Inputs))

	// fill defaults first
	for name, in := range spec.Inputs {
		if in.Default.IsZero() {
			continue
		}
		var defaultVal any
		if err := in.Default.Decode(&defaultVal); err != nil {
			return nil, fmt.Errorf("decode workflow_call input %q default: %w", name, err)
		}
		v, err := parseWorkflowCallInput(name, in.Type, defaultVal)
		if err != nil {
			return nil, err
		}
		resolved[name] = v
	}

	for k, raw := range evaluated {
		inputSpec, ok := spec.Inputs[k]
		if !ok {
			// ignore unknown "with:" keys
			continue
		}
		converted, err := parseWorkflowCallInput(k, inputSpec.Type, raw)
		if err != nil {
			return nil, err
		}
		resolved[k] = converted
	}

	for name, in := range spec.Inputs {
		if !in.Required {
			continue
		}
		// resolved[name] is set when caller provided it OR when spec has a non-zero default - both satisfy "required".
		if _, ok := resolved[name]; ok {
			continue
		}
		return nil, fmt.Errorf("workflow_call input %q is required", name)
	}

	return resolved, nil
}

func parseWorkflowCallInput(name string, typ InputType, v any) (any, error) {
	switch typ {
	case InputTypeString:
		return toString(v), nil
	case InputTypeBoolean:
		// strict type matching: a boolean input only accepts a native bool, not a "true"/"false" string
		if b, ok := v.(bool); ok {
			return b, nil
		}
		return false, fmt.Errorf("workflow_call input %q expects boolean", name)
	case InputTypeNumber:
		// strict type matching: a number input rejects "123"/"3.14" strings.
		if _, isString := v.(string); isString {
			return 0.0, fmt.Errorf("workflow_call input %q expects number", name)
		}
		return util.ToFloat64(v)
	default:
		return nil, fmt.Errorf("workflow_call input %q has unsupported type %q", name, typ)
	}
}

// SecretsInherit is the literal keyword used in a caller's `secrets: inherit` directive
const SecretsInherit = "inherit"

// callerSecretValueRegexp matches the `${{ secrets.NAME }}` form expected for each value in a caller's `secrets:` mapping.
var callerSecretValueRegexp = regexp.MustCompile(`^\s*\$\{\{\s*secrets\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}\s*$`)

// ParseCallerSecrets decodes a caller's "secrets:" YAML node into one of two forms:
//   - inherit == true: the caller wrote `secrets: inherit`; mapping is nil
//   - inherit == false, mapping == {alias: source_name}: explicit mapping. Each value must be of the form `${{ secrets.NAME }}`.
//
// Both alias and source name are upper-cased: secret names are case-insensitive (matching GitHub),
// and Gitea stores secrets upper-cased, so this keeps lookups and schema validation consistent.
func ParseCallerSecrets(node yaml.Node) (inherit bool, mapping map[string]string, err error) {
	if node.IsZero() {
		return false, nil, nil
	}
	if node.Kind == yaml.ScalarNode && strings.TrimSpace(node.Value) == SecretsInherit {
		return true, nil, nil
	}
	if node.Kind != yaml.MappingNode {
		return false, nil, errors.New("invalid secrets: section, expected mapping or 'inherit'")
	}
	out := make(map[string]string, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i]
		v := node.Content[i+1]
		var sv string
		if err := v.Decode(&sv); err != nil {
			return false, nil, fmt.Errorf("decode secret %q: %w", k.Value, err)
		}
		matches := callerSecretValueRegexp.FindStringSubmatch(sv)
		if len(matches) != 2 {
			return false, nil, fmt.Errorf("caller secret %q value must be of the form ${{ secrets.NAME }}", k.Value)
		}
		out[strings.ToUpper(k.Value)] = strings.ToUpper(matches[1])
	}
	return false, out, nil
}

// ValidateCallerSecrets checks a caller's parsed explicit-mapping `secrets:` against the called workflow's declared `on.workflow_call.secrets` schema.
func ValidateCallerSecrets(spec *WorkflowCallSpec, mapping map[string]string) error {
	if spec == nil {
		return errors.New("ValidateCallerSecrets: nil workflow_call spec")
	}
	// Secret names are case-insensitive, so compare declared names and caller aliases upper-cased.
	declaredNames := make(container.Set[string], len(spec.Secrets))
	for name := range spec.Secrets {
		declaredNames.Add(strings.ToUpper(name))
	}
	provided := make(container.Set[string], len(mapping))
	for alias := range mapping {
		up := strings.ToUpper(alias)
		provided.Add(up)
		if !declaredNames.Contains(up) {
			return fmt.Errorf("caller secret %q is not declared in the called workflow's on.workflow_call.secrets", alias)
		}
	}
	for name, sec := range spec.Secrets {
		if sec.Required && !provided.Contains(strings.ToUpper(name)) {
			return fmt.Errorf("required secret %q is not provided by the caller", name)
		}
	}
	return nil
}

// EvaluateWorkflowCallOutputs evaluates a called workflow's "on.workflow_call.outputs.<name>.value" expressions against the provided contexts.
func EvaluateWorkflowCallOutputs(spec *WorkflowCallSpec, gitCtx *model.GithubContext, vars map[string]string, inputs map[string]any, jobOutputs JobOutputs) (map[string]string, error) {
	if spec == nil || len(spec.Outputs) == 0 {
		return map[string]string{}, nil
	}

	jobsCtx := make(map[string]*model.WorkflowCallResult, len(jobOutputs))
	for jobID, outputs := range jobOutputs {
		jobsCtx[jobID] = &model.WorkflowCallResult{Outputs: outputs}
	}

	// See `on.workflow_call.outputs.<output_id>.value` in https://docs.github.com/en/actions/reference/workflows-and-actions/contexts#context-availability
	env := &exprparser.EvaluationEnvironment{
		Github: gitCtx,
		Jobs:   &jobsCtx,
		Vars:   vars,
		Inputs: inputs,
	}
	interpreter := exprparser.NewInterpeter(env, exprparser.Config{})

	out := make(map[string]string, len(spec.Outputs))
	for name, o := range spec.Outputs {
		v, err := evaluateWorkflowCallOutputValue(interpreter, o.Value)
		if err != nil {
			return nil, fmt.Errorf("workflow_call output %q: %w", name, err)
		}
		out[name] = v
	}
	return out, nil
}

func evaluateWorkflowCallOutputValue(interpreter exprparser.Interpreter, value string) (string, error) {
	if !strings.Contains(value, "${{") || !strings.Contains(value, "}}") {
		return value, nil
	}
	expr, err := rewriteSubExpression(value, true)
	if err != nil {
		return "", err
	}
	evaluated, err := interpreter.Evaluate(expr, exprparser.DefaultStatusCheckNone)
	if err != nil {
		return "", err
	}
	return toString(evaluated), nil
}

func toString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", s)
	}
}
