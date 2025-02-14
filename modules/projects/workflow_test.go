// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseWorkflow(t *testing.T) {
	workflowFile := `
name: Test Workflow
on:
	item_added_to_project:
		types: [issue, pull_request]
		action:
			- set_value: "status=Todo"

	item_closed:
		types: [issue, pull_request]
		action:
			- remove_label: ""

	item_reopened:
		action:

	code_changes_requested:
		action:

	code_review_approved:
		action:

	pull_request_merged:
		action:

	auto_add_to_project:
		action:
`

	wf, err := ParseWorkflow(workflowFile)
	assert.NoError(t, err)

	assert.Equal(t, "Test Workflow", wf.Name)
}
