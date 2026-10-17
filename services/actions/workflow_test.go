// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/actionslib/pkg/model"

	"github.com/stretchr/testify/assert"
)

func TestCoerceDispatchInputTypes(t *testing.T) {
	dispatch := &model.WorkflowDispatch{
		Inputs: map[string]model.WorkflowDispatchInput{
			"build_server": {Type: "boolean"},
			"dry_run":      {Type: "boolean"},
			"already_bool": {Type: "boolean"},
			"yaml_true":    {Type: "boolean"},
			"yaml_truthy":  {Type: "boolean"},
			"version":      {Type: "string"},
		},
	}

	inputs := map[string]any{
		// dispatch callbacks fill booleans as strconv.FormatBool(...) strings
		"build_server": "true",
		"dry_run":      "false",
		// already-native booleans are passed through unchanged (coercion is idempotent)
		"already_bool": true,
		// source text of `default: True` and `default: yes`, only the former is a YAML 1.2 boolean
		"yaml_true":   "True",
		"yaml_truthy": "yes",
		// non-boolean inputs must be left untouched
		"version": "1.2.3",
	}

	coerceDispatchInputTypes(dispatch, inputs)

	// Regression: without coercion these stay strings, and a server-side needs-gated
	// job `if: inputs.build_server == true` never matches, leaving the job blocked.
	assert.Equal(t, true, inputs["build_server"])
	assert.Equal(t, false, inputs["dry_run"])
	assert.Equal(t, true, inputs["already_bool"])
	assert.Equal(t, true, inputs["yaml_true"])
	assert.Equal(t, false, inputs["yaml_truthy"])
	assert.Equal(t, "1.2.3", inputs["version"])

	// `github.event.inputs` mirrors them as normalized strings
	assert.Equal(t, map[string]any{
		"build_server": "true",
		"dry_run":      "false",
		"already_bool": "true",
		"yaml_true":    "true",
		"yaml_truthy":  "false",
		"version":      "1.2.3",
	}, dispatchEventInputs(inputs))
}
