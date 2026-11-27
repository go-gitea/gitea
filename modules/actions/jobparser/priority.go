// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"fmt"

	"go.yaml.in/yaml/v4"
)

// Levels of the Gitea-specific workflow key `priority`, runners get waiting jobs of a higher level first.
const (
	PriorityLow    = -1
	PriorityNormal = 0
	PriorityHigh   = 1
)

// ParsePriority maps the value of the workflow key `priority` to its level, an empty value being `normal`.
func ParsePriority(value string) (int, error) {
	switch value {
	case "high":
		return PriorityHigh, nil
	case "", "normal":
		return PriorityNormal, nil
	case "low":
		return PriorityLow, nil
	}
	return PriorityNormal, fmt.Errorf("priority: %q is not high, normal or low", value)
}

// validatePriority rejects a workflow whose top-level `priority` is not one ParsePriority accepts.
func validatePriority(doc *yaml.Node) error {
	var header struct {
		Priority yaml.Node `yaml:"priority"`
	}
	if err := decodeYamlDoc(doc, &header); err != nil {
		return err
	}
	if header.Priority.Kind == 0 {
		return nil
	}
	if header.Priority.Kind != yaml.ScalarNode {
		return fmt.Errorf("priority: expected one of high, normal or low (line %d)", header.Priority.Line)
	}
	_, err := ParsePriority(header.Priority.Value)
	return err
}
