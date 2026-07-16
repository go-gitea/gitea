// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.com/gitea/runner/act/model"
	"github.com/stretchr/testify/assert"
)

func TestCoerceDispatchInputTypes(t *testing.T) {
	dispatch := &model.WorkflowDispatch{
		Inputs: map[string]model.WorkflowDispatchInput{
			"build_server": {Type: "boolean"},
			"dry_run":      {Type: "boolean"},
			"already_bool": {Type: "boolean"},
			"version":      {Type: "string"},
		},
	}

	inputs := map[string]any{
		// dispatch callbacks fill booleans as strconv.FormatBool(...) strings
		"build_server": "true",
		"dry_run":      "false",
		// already-native booleans are passed through unchanged (coercion is idempotent)
		"already_bool": true,
		// non-boolean inputs must be left untouched
		"version": "1.2.3",
	}

	coerceDispatchInputTypes(dispatch, inputs)

	// Regression: without coercion these stay strings, and a server-side needs-gated
	// job `if: inputs.build_server == true` never matches, leaving the job blocked.
	assert.Equal(t, true, inputs["build_server"])
	assert.Equal(t, false, inputs["dry_run"])
	assert.Equal(t, true, inputs["already_bool"])
	assert.Equal(t, "1.2.3", inputs["version"])
}
