// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"testing"

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

	payload, err := sws[0].Marshal()
	require.NoError(t, err)

	// the serialized single workflow must be parseable again -- this is what the
	// server does in EvaluateJobConcurrencyFillModel -> ParseJob. Before the fix
	// Marshal emitted `run: |4`, which failed here and left the job blocked.
	_, err = Parse(payload)
	require.NoError(t, err, "serialized single workflow must round-trip; got payload:\n%s", payload)
}
