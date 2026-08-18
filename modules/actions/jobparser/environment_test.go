// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentEnvironmentName(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantEnv string
	}{
		{
			name:    "no environment key",
			yaml:    "on: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n",
			wantEnv: "",
		},
		{
			name:    "scalar string environment",
			yaml:    "on: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n    environment: production\n    steps:\n      - run: echo hi\n",
			wantEnv: "production",
		},
		{
			name:    "object environment with name",
			yaml:    "on: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n    environment:\n      name: staging\n      url: https://staging.example.com\n    steps:\n      - run: echo hi\n",
			wantEnv: "staging",
		},
		{
			name:    "object environment name only",
			yaml:    "on: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n    environment:\n      name: preview\n    steps:\n      - run: echo hi\n",
			wantEnv: "preview",
		},
		{
			name:    "expression resolving to nothing",
			yaml:    "on: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n    environment: ${{ vars.UNSET }}\n    steps:\n      - run: echo hi\n",
			wantEnv: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflows, err := Parse([]byte(tt.yaml))
			require.NoError(t, err)
			require.Len(t, workflows, 1)
			_, job := workflows[0].Job()
			require.NotNil(t, job)
			assert.Equal(t, tt.wantEnv, job.DeploymentEnvironmentName())
		})
	}
}

func TestDeploymentEnvironmentNameInterpolation(t *testing.T) {
	t.Run("vars", func(t *testing.T) {
		workflows, err := Parse(
			[]byte("on: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n    environment: ${{ vars.STAGE }}\n    steps:\n      - run: echo hi\n"),
			WithVars(map[string]string{"STAGE": "staging"}))
		require.NoError(t, err)
		require.Len(t, workflows, 1)
		_, job := workflows[0].Job()
		assert.Equal(t, "staging", job.DeploymentEnvironmentName())
	})

	// the placeholder keeps its expression raw until expansion, so it must not name an environment
	t.Run("deferred matrix", func(t *testing.T) {
		workflows, err := Parse([]byte("on: push\njobs:\n  setup:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n  deploy:\n    needs: setup\n    runs-on: ubuntu-latest\n    strategy:\n      matrix:\n        target: ${{ fromJson(needs.setup.outputs.envs) }}\n    environment: ${{ matrix.target }}\n    steps:\n      - run: echo hi\n"))
		require.NoError(t, err)
		_, job := workflows[1].Job()
		require.True(t, HasDeferredMatrix(job))
		assert.Empty(t, job.DeploymentEnvironmentName())
	})

	t.Run("matrix", func(t *testing.T) {
		workflows, err := Parse([]byte("on: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n    strategy:\n      matrix:\n        target: [staging, production]\n    environment:\n      name: ${{ matrix.target }}\n    steps:\n      - run: echo hi\n"))
		require.NoError(t, err)
		require.Len(t, workflows, 2)
		got := make([]string, 0, len(workflows))
		for _, w := range workflows {
			_, job := w.Job()
			got = append(got, job.DeploymentEnvironmentName())
		}
		assert.ElementsMatch(t, []string{"staging", "production"}, got)
	})
}
