// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseResolvesAliases(t *testing.T) {
	got, err := Parse([]byte(`on: push
env: &common_env
  SHARED: "1"
jobs:
  a:
    runs-on: linux
    env: *common_env
    steps: &common_steps [{run: echo hi}]
  b:
    runs-on: linux
    env: *common_env
    steps: *common_steps
`))
	require.NoError(t, err)
	require.Len(t, got, 2)

	for _, workflow := range got {
		_, job := workflow.Job()
		var env map[string]string
		require.NoError(t, job.Env.Decode(&env))
		assert.Equal(t, map[string]string{"SHARED": "1"}, env)
		require.Len(t, job.Steps, 1)

		payload, err := workflow.Marshal()
		require.NoError(t, err)
		assert.NotContains(t, string(payload), "common_")
	}
}

func TestParseRejectsAliases(t *testing.T) {
	job := func(body string) []byte {
		return []byte("on: push\njobs:\n  a:\n    runs-on: linux\n" + body)
	}

	for _, tt := range []struct {
		name, wantErr string
		content       []byte
	}{
		{
			name: "nested aliases exceed the node limit",
			content: []byte(`on: push
x0: &x0 [1, 2, 3, 4, 5, 6, 7, 8, 9]
x1: &x1 [*x0, *x0, *x0, *x0, *x0, *x0, *x0, *x0, *x0]
x2: &x2 [*x1, *x1, *x1, *x1, *x1, *x1, *x1, *x1, *x1]
x3: &x3 [*x2, *x2, *x2, *x2, *x2, *x2, *x2, *x2, *x2]
x4: &x4 [*x3, *x3, *x3, *x3, *x3, *x3, *x3, *x3, *x3]
jobs: {a: {runs-on: linux, steps: [{run: echo}]}}
`),
			wantErr: "maximum YAML nodes exceeded",
		},
		{
			name:    "anchor aliased from inside itself",
			content: job("    steps: &s [{run: echo}, *s]\n"),
			wantErr: "maximum YAML nodes exceeded",
		},
		{
			name:    "merge key",
			content: job("    env: &e {X: \"1\"}\n    container:\n      image: alpine\n      env:\n        <<: *e\n"),
			wantErr: "merge keys (`<<`) are not supported",
		},
		{
			name:    "alias before its anchor",
			content: job("    env: *e\n    container: {image: alpine, env: &e {X: \"1\"}}\n"),
			wantErr: "unknown anchor 'e' referenced",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.content)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
