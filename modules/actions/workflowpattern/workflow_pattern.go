// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package workflowpattern

import (
	"strings"

	"code.gitea.io/gitea/modules/glob"
)

type WorkflowPattern struct {
	negative bool
	glob     glob.Glob
}

func CompilePatterns(patterns ...string) ([]*WorkflowPattern, error) {
	ret := make([]*WorkflowPattern, 0, len(patterns))
	for _, pattern := range patterns {
		cp, err := glob.CompileWorkflow(pattern)
		if err != nil {
			return nil, err
		}
		ret = append(ret, &WorkflowPattern{glob: cp, negative: strings.HasPrefix(pattern, "!")})
	}
	return ret, nil
}

// Skip returns true if the workflow should be skipped per paths/branches semantics.
func Skip(sequence []*WorkflowPattern, input []string) bool {
	allSkipped := true
	for _, file := range input {
		shouldSkip := true
		for _, item := range sequence {
			if item.negative {
				// "!README.md" doesn't match "README.md", so "README.md" should be skipped
				// "!README.md" matches "help.md" but it shouldn't affect "skip or not", because "help.md" might have been skipped by other rules like "docs/*.md"
				if !item.glob.Match(file) {
					shouldSkip = true
				}
			} else if item.glob.Match(file) {
				// if "*.md" matches "help.md" so it shouldn't be skipped
				shouldSkip = false
			}
		}
		allSkipped = allSkipped && shouldSkip
	}
	return len(sequence) > 0 && allSkipped
}

// Filter returns true if the workflow should be skipped per paths-ignore/branches-ignore semantics.
func Filter(sequence []*WorkflowPattern, input []string) bool {
	for _, file := range input {
		anyMatched := false
		for _, item := range sequence {
			if anyMatched = item.glob.Match(file); anyMatched {
				break
			}
		}
		if !anyMatched {
			return false
		}
	}
	return len(sequence) != 0
}
