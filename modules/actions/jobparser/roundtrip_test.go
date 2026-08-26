// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"bytes"
	"testing"

	actmodel "gitea.dev/actionslib/pkg/model"

	"github.com/stretchr/testify/require"
)

// A step whose `run:` block starts with blank lines must still survive the
// Parse -> SingleWorkflow.Marshal -> Parse round-trip. Previously Marshal used a
// different indentation than SetJob, which made the encoder emit the block scalar
// with a wrong explicit indentation indicator (`run: |4`) that no longer parsed;
// the job then stayed silently blocked during concurrency evaluation.
func TestSingleWorkflowRoundTripRunBlockLeadingBlankLines(t *testing.T) {
	const wf = `name: demo
on:
  workflow_call:
    inputs:
      app_name:
        type: string
        required: true
jobs:
  build:
    name: build
    env:
      IMAGE_TAG: ${{ inputs.app_name }}
    runs-on: ubuntu-latest
    steps:
      - if: ${{ inputs.app_name != '' }}
        name: packages
        run: |


          echo start
          echo done
`
	sws, err := Parse([]byte(wf))
	require.NoError(t, err)
	require.Len(t, sws, 1)

	// pin the original run block as the baseline
	_, origJob := sws[0].Job()
	require.Len(t, origJob.Steps, 1)
	const wantRun = "\n\necho start\necho done\n"
	require.Equal(t, wantRun, origJob.Steps[0].Run)

	payload, err := sws[0].Marshal()
	require.NoError(t, err)

	// the serialized single workflow must be parseable again -- this is what the
	// server does in EvaluateJobConcurrencyFillModel -> ParseJob. Before the fix
	// Marshal emitted `run: |4`, which failed here and left the job blocked.
	roundTripped, err := Parse(payload)
	require.NoError(t, err, "serialized single workflow must round-trip; got payload:\n%s", payload)
	require.Len(t, roundTripped, 1)

	// the round-trip must preserve the run block byte-for-byte
	_, gotJob := roundTripped[0].Job()
	require.Len(t, gotJob.Steps, 1)
	require.Equal(t, wantRun, gotJob.Steps[0].Run, "round-trip must preserve run content; got payload:\n%s", payload)
}

// A step-level `continue-on-error` may hold an expression reading the steps context, which only the
// runner can evaluate. The server must therefore parse it without deciding it and hand it to the
// runner unchanged: typing it as a bool used to abort decoding of the whole `jobs:` node.
func TestSingleWorkflowRoundTripStepContinueOnError(t *testing.T) {
	const wf = `name: demo
on: push
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - id: quarantine
        run: echo "q=true" >> "$GITHUB_OUTPUT"
      - run: exit 1
        continue-on-error: ${{ steps.quarantine.outputs.q == 'true' }}
      - run: exit 1
        continue-on-error: true
`
	sws, err := Parse([]byte(wf))
	require.NoError(t, err)
	require.Len(t, sws, 1)

	payload, err := sws[0].Marshal()
	require.NoError(t, err)

	// the payload is what the runner reads back as act's model.Workflow, whose step keeps the raw
	// string it evaluates at step execution time
	rw, err := actmodel.ReadWorkflow(bytes.NewReader(payload))
	require.NoError(t, err, "payload:\n%s", payload)
	steps := rw.Jobs["job1"].Steps
	require.Len(t, steps, 3)
	require.Empty(t, steps[0].RawContinueOnError)
	require.Equal(t, "${{ steps.quarantine.outputs.q == 'true' }}", steps[1].RawContinueOnError, "payload:\n%s", payload)
	require.Equal(t, "true", steps[2].RawContinueOnError, "payload:\n%s", payload)
}
