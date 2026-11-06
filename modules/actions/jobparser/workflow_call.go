// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gitea.dev/actionslib/pkg/model"

	"go.yaml.in/yaml/v4"
)

// ResolveCallerInputs evaluates the caller's `with` against its own contexts and types it by the called workflow's inputs.
func ResolveCallerInputs(
	jobID string,
	job *Job,
	config *model.WorkflowCall,
	gitCtx map[string]any,
	results map[string]*JobResult,
	vars map[string]string,
	inputs map[string]any,
) (map[string]any, error) {
	if job.With.Kind != 0 && job.With.Kind != yaml.MappingNode {
		return nil, errors.New("caller with must be a mapping")
	}
	evaluator, err := newJobEvaluator(jobID, job, gitCtx, results, vars, inputs)
	if err != nil {
		return nil, fmt.Errorf("get caller %q matrix: %w", jobID, err)
	}
	var with map[string]any
	if err := model.DecodeEvaluated("with", job.With, evaluator.EvaluateYamlNode, &with); err != nil {
		return nil, err
	}
	declared := make(map[string]model.WorkflowCallInput, len(config.Inputs))
	for name, input := range config.Inputs {
		declared[strings.ToLower(name)] = input
	}
	for name, value := range with {
		input, ok := declared[strings.ToLower(name)]
		if !ok {
			continue // ignored as in released Gitea, github.com rejects an undeclared input
		}
		switch input.Type {
		case "boolean":
			if _, ok := value.(bool); !ok {
				return nil, fmt.Errorf("input %s: expected a boolean, got %T", name, value)
			}
		case "number":
			switch value.(type) {
			case float64, int, int64, uint64:
			default:
				return nil, fmt.Errorf("input %s: expected a number, got %T", name, value)
			}
		}
	}
	return evaluator.ResolveWorkflowCallInputs(config, with)
}

// ParseWorkflowCallConfig reads the workflow_call configuration of a called workflow, decoding only its `on`.
func ParseWorkflowCallConfig(content []byte) (*model.WorkflowCall, error) {
	var workflow struct {
		RawOn yaml.Node `yaml:"on"`
	}
	if err := decodeResolved(content, &workflow); err != nil {
		return nil, err
	}
	return (&model.Workflow{RawOn: workflow.RawOn}).ParseWorkflowCallConfig()
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
