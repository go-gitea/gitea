// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"gitea.dev/actionslib/pkg/expreval"
	"gitea.dev/actionslib/pkg/exprparser"
	"gitea.dev/actionslib/pkg/model"

	"go.yaml.in/yaml/v4"
)

// jobConditionContexts are what github.com gives `jobs.<job_id>.if`, which it decides before the matrix, plus the `gitea` alias.
var jobConditionContexts = []string{"github", "gitea", "needs", "vars", "inputs"}

func ValidateWorkflowStatic(content []byte) ([]*Event, error) {
	doc, err := resolveYamlAliases(content)
	if err != nil {
		return nil, err
	}
	// Keep unknown and case-distinct keys outside of jobs accepted for existing Gitea workflows.
	workflow, err := readWorkflowDoc(doc)
	if err != nil {
		return nil, err
	}
	events, err := ParseRawOn(&workflow.RawOn)
	if err != nil {
		return nil, err
	}
	if err := validateWorkflowStructure(workflow); err != nil {
		return nil, err
	}
	if err := validateJobKinds(doc); err != nil {
		return nil, err
	}
	var header struct {
		RunName string `yaml:"run-name"`
	}
	if err := decodeYamlDoc(doc, &header); err != nil {
		return nil, err
	}
	if unavailable := unavailableContext(header.RunName, []string{"github", "gitea", "inputs", "vars"}); unavailable != "" {
		return nil, fmt.Errorf("run-name: Unrecognized named-value: '%s'", unavailable)
	}
	return events, nil
}

func validateJobConditions(workflow *model.Workflow) error {
	for id, job := range workflow.Jobs {
		if job == nil {
			continue
		}
		if unavailable := unavailableContext(IfExpression(job.If.Value), jobConditionContexts); unavailable != "" {
			return fmt.Errorf("job %s: Unrecognized named-value: '%s'", id, unavailable)
		}
	}
	return nil
}

func unavailableContext(expression string, allowed []string) string {
	var unavailable string
	expreval.Match(expression, func(node exprparser.ExprNode) bool {
		if variable, ok := node.(*exprparser.VariableNode); ok && !slices.ContainsFunc(allowed, func(name string) bool { return exprparser.OrdinalIgnoreCaseEqual(name, variable.Name) }) {
			unavailable = variable.Name
		}
		return unavailable != ""
	})
	return unavailable
}

func validateWorkflowStructure(workflow *model.Workflow) error {
	if len(workflow.Jobs) == 0 {
		return errors.New("the workflow must contain at least one job")
	}
	for id, job := range workflow.Jobs {
		if job == nil {
			return fmt.Errorf("job %q has no configuration", id)
		}
		for _, dependency := range job.Needs() {
			if _, ok := workflow.Jobs[dependency]; !ok {
				return fmt.Errorf("job %q needs unknown job %q", id, dependency)
			}
		}
	}
	visited := make(map[string]bool, len(workflow.Jobs))
	visiting := make(map[string]bool, len(workflow.Jobs))
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("job %q has a dependency cycle", id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, dependency := range workflow.Jobs[id].Needs() {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		delete(visiting, id)
		visited[id] = true
		return nil
	}
	for id := range workflow.Jobs {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

// job keys of github.com's workflow schema, by the kind of job allowing them
var (
	stepsJobKeys  = []string{"cancel-timeout-minutes", "container", "continue-on-error", "defaults", "env", "environment", "outputs", "runs-on", "services", "snapshot", "steps", "timeout-minutes"}
	callerJobKeys = []string{"secrets", "uses", "with"}
	sharedJobKeys = []string{"concurrency", "if", "name", "needs", "permissions", "strategy"}
)

// validateJobKinds applies github.com's job kinds, decided by the first kind-specific key.
func validateJobKinds(doc *yaml.Node) error {
	jobs := mappingValue(doc.Content[0], "jobs")
	for i := 0; i+1 < len(jobs.Content); i += 2 {
		id, job := jobs.Content[i].Value, jobs.Content[i+1]
		var required string
		for j := 0; j+1 < len(job.Content); j += 2 {
			key := job.Content[j].Value
			isStepsKey, isCallerKey := slices.Contains(stepsJobKeys, key), slices.Contains(callerJobKeys, key)
			switch {
			case required == "runs-on" && isCallerKey, required == "uses" && isStepsKey, !isStepsKey && !isCallerKey && !slices.Contains(sharedJobKeys, key):
				return fmt.Errorf("job %s: Unexpected value '%s'", id, key)
			case required == "" && isStepsKey:
				required = "runs-on"
			case required == "" && isCallerKey:
				required = "uses"
			}
		}
		if required == "" {
			keys := slices.Concat(stepsJobKeys, callerJobKeys)
			slices.Sort(keys)
			return fmt.Errorf("job %s: There's not enough info to determine what you meant. Add one of these properties: %s", id, strings.Join(keys, ", "))
		}
		value := mappingValue(job, required)
		if value == nil {
			return fmt.Errorf("job %s: Required property is missing: %s", id, required)
		}
		if required != "runs-on" {
			continue
		}
		if problem := runsOnProblem(value); problem != "" {
			return fmt.Errorf("job %s: runs-on: %s", id, problem)
		}
	}
	return nil
}

// runsOnProblem returns github.com's schema error for a runs-on, "" if valid.
func runsOnProblem(node *yaml.Node) string {
	if node.Kind != yaml.MappingNode {
		return runsOnLabelsProblem(node)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		var problem string
		switch key := node.Content[i].Value; key {
		case "labels":
			problem = runsOnLabelsProblem(node.Content[i+1])
		case "group":
			problem = runsOnGroupProblem(node.Content[i+1])
		default:
			problem = fmt.Sprintf("Unexpected value '%s'", key)
		}
		if problem != "" {
			return problem
		}
	}
	return ""
}

func runsOnLabelsProblem(node *yaml.Node) string {
	if node.Kind != yaml.SequenceNode {
		return nonEmptyStringProblem(node)
	}
	for _, label := range node.Content {
		if problem := nonEmptyStringProblem(label); problem != "" {
			return problem
		}
	}
	return ""
}

func runsOnGroupProblem(node *yaml.Node) string {
	if problem := nonEmptyStringProblem(node); problem != "" || hasExpression(node.Value) {
		return problem
	}
	switch prefix, name, found := strings.Cut(node.Value, "/"); {
	case found && name == "":
		return fmt.Sprintf("Invalid runs-on group name '%s'.", node.Value)
	case found && (strings.Contains(name, "/") || !slices.Contains([]string{"org", "organization", "ent", "enterprise"}, prefix)):
		return fmt.Sprintf("Invalid runs-on group name '%s'. Please use 'organization/' or 'enterprise/' prefix to target a single runner group.", node.Value)
	}
	return ""
}

// nonEmptyStringProblem mirrors github.com's non-empty-string, which also accepts non-string scalars.
func nonEmptyStringProblem(node *yaml.Node) string {
	switch {
	case node.Kind == yaml.SequenceNode:
		return "A sequence was not expected"
	case node.Kind == yaml.MappingNode:
		return "A mapping was not expected"
	case node.Value == "" || node.ShortTag() == "!!null":
		return "Unexpected value ''"
	}
	return ""
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
