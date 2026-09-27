// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"errors"
	"fmt"
	"slices"

	"gitea.dev/actionslib/pkg/expreval"
	"gitea.dev/actionslib/pkg/exprparser"
	"gitea.dev/actionslib/pkg/model"
)

// jobConditionContexts are what github.com gives `jobs.<job_id>.if`, which it decides before the matrix, plus the `gitea` alias.
var jobConditionContexts = []string{"github", "gitea", "needs", "vars", "inputs"}

func ValidateWorkflowStatic(content []byte) ([]*Event, error) {
	doc, err := resolveYamlAliases(content)
	if err != nil {
		return nil, err
	}
	// Keep unknown and case-distinct keys accepted for existing Gitea workflows.
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
		// a job without runs-on is accepted and runs on any runner, github.com rejects it
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
