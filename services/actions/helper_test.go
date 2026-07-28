// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"testing"

	actions_model "gitea.dev/models/actions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateJobIfDefersMatrixExpression(t *testing.T) {
	// A placeholder's `if:` reading `matrix.*` can only be decided per combination once the matrix is expanded.

	emptyRun := &actions_model.ActionRun{} // if the gate is removed, the emptyRun will cause an error
	deferredJob := func(ifExpr string) *actions_model.ActionRunJob {
		return &actions_model.ActionRunJob{
			ID: 1, JobID: "build", Needs: []string{"setup"}, IsMatrixDeferred: true,
			WorkflowPayload: fmt.Appendf(nil, `name: test
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    if: %s
    strategy:
      matrix:
        value: ${{ fromJson(needs.setup.outputs.m) }}
    steps:
      - run: echo
`, ifExpr),
		}
	}

	for _, ifExpr := range []string{
		// Both hold only for a real combination, so neither can be decided before the matrix expands.
		"${{ matrix.value == 1 }}",
		"${{ contains(fromJson('[1]'), matrix.value) }}",
	} {
		t.Run(ifExpr, func(t *testing.T) {
			got, err := evaluateJobIf(t.Context(), emptyRun, nil, deferredJob(ifExpr), nil, true)
			require.NoError(t, err)
			assert.True(t, got, "must reach the expansion that gates each combination on its own values")

			// The fallback is the needs gate, so a job whose needs did not succeed is still skipped.
			got, err = evaluateJobIf(t.Context(), emptyRun, nil, deferredJob(ifExpr), nil, false)
			require.NoError(t, err)
			assert.False(t, got)
		})
	}
}
