// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import "gitea.dev/actionslib/pkg/model"

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
	if err := model.ValidateWorkflow(doc); err != nil {
		return nil, err
	}
	return events, nil
}
