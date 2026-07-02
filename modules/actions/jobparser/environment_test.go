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
