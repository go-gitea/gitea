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

	for _, tt := range []struct {
		ifExpr string
		// wantNeedsFailed is what the `if:` decides when the needs did not all succeed.
		wantNeedsFailed bool
	}{
		// These hold only for a real combination, so none can be decided before the matrix expands,
		// and the fallback is the needs gate: a job whose needs did not succeed is still skipped.
		{ifExpr: "${{ matrix.value == 1 }}"},
		{ifExpr: "matrix.value == 1"}, // an `if:` may omit the `${{ }}`
		// always() asks to run whatever the needs did, so it must not fall back to their gate.
		{ifExpr: "${{ always() && matrix.value == 1 }}", wantNeedsFailed: true},
	} {
		t.Run(tt.ifExpr, func(t *testing.T) {
			got, err := evaluateJobIf(t.Context(), emptyRun, nil, deferredJob(tt.ifExpr), nil, true)
			require.NoError(t, err)
			assert.True(t, got, "must reach the expansion that gates each combination on its own values")

			got, err = evaluateJobIf(t.Context(), emptyRun, nil, deferredJob(tt.ifExpr), nil, false)
			require.NoError(t, err)
			assert.Equal(t, tt.wantNeedsFailed, got)
		})
	}
}
