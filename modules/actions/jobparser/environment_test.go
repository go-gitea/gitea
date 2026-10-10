// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentEnvironmentName(t *testing.T) {
	const setupJob = "  setup:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n"
	const deployHead = "  deploy:\n    runs-on: ubuntu-latest\n"
	const steps = "    steps:\n      - run: echo hi\n"
	tests := []struct {
		name         string
		jobs         string
		opts         []ParseOption
		want         []string // one entry per parsed job, sorted
		wantDeferred bool
	}{
		{name: "none", jobs: deployHead + steps, want: []string{""}},
		{name: "scalar", jobs: deployHead + "    environment: production\n" + steps, want: []string{"production"}},
		{name: "object", jobs: deployHead + "    environment:\n      name: staging\n      url: https://staging.example.com\n" + steps, want: []string{"staging"}},
		{name: "expression resolving to nothing", jobs: deployHead + "    environment: ${{ vars.UNSET }}\n" + steps, want: []string{""}},
		{
			name: "vars", jobs: deployHead + "    environment: ${{ vars.STAGE }}\n" + steps,
			opts: []ParseOption{WithVars(map[string]string{"STAGE": "staging"})}, want: []string{"staging"},
		},
		{
			name: "matrix", jobs: deployHead + "    strategy:\n      matrix:\n        target: [staging, production]\n    environment:\n      name: ${{ matrix.target }}\n" + steps,
			want: []string{"production", "staging"},
		},
		{
			name: "deferred matrix", jobs: setupJob + deployHead + "    needs: setup\n    strategy:\n      matrix:\n        target: ${{ fromJson(needs.setup.outputs.envs) }}\n    environment: ${{ matrix.target }}\n" + steps,
			want: []string{"", ""}, wantDeferred: true,
		},
		{
			name: "environment read from needs alone", jobs: setupJob + deployHead + "    needs: setup\n    environment: ${{ needs.setup.outputs.env }}\n" + steps,
			want: []string{"", ""}, wantDeferred: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflows, err := Parse([]byte("on: push\njobs:\n"+tt.jobs), tt.opts...)
			require.NoError(t, err)
			var got []string
			deferred := false
			for _, w := range workflows {
				_, job := w.Job()
				got = append(got, job.DeploymentEnvironmentName())
				deferred = deferred || HasDeferredMatrix(job)
			}
			slices.Sort(got)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantDeferred, deferred)
		})
	}
}
