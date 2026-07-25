// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/url"
	"strings"
	"testing"
	"time"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDynamicMatrixEvaluation covers a job whose matrix references ${{ needs.*.outputs.* }}: it is
// planned as a single placeholder and expanded once its dependency completes. `build` exercises the
// expansion, `report` that a downstream job sees the combinations' outputs, and `gated` that a job
// the `if:` skips is never expanded and never dispatched.
func TestDynamicMatrixEvaluation(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-dynamic-matrix-eval", false)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, apiRepo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		const workflow = `name: dynamic-matrix
on:
  push:
    paths: ['.gitea/workflows/dynamic-matrix.yml']
jobs:
  generate:
    runs-on: ubuntu-latest
    outputs:
      matrix: ${{ steps.set.outputs.matrix }}
    steps:
      - id: set
        run: echo "matrix=[1,2]" >> "$GITHUB_OUTPUT"
  build:
    needs: [generate]
    runs-on: ubuntu-latest
    outputs:
      result: ${{ steps.out.outputs.result }}
    strategy:
      matrix:
        value: ${{ fromJson(needs.generate.outputs.matrix) }}
    steps:
      - id: out
        run: echo "result=built-${{ matrix.value }}" >> "$GITHUB_OUTPUT"
  gated:
    needs: [generate]
    if: false
    runs-on: ubuntu-latest
    strategy:
      matrix:
        value: ${{ fromJson(needs.generate.outputs.matrix) }}
    steps:
      - run: echo "${{ matrix.value }}"
  report:
    needs: [build]
    runs-on: ubuntu-latest
    steps:
      - run: echo '${{ toJSON(needs.build.outputs) }}'
`
		opts := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, "create dynamic-matrix.yml", workflow)
		createWorkflowFile(t, token, user2.Name, apiRepo.Name, ".gitea/workflows/dynamic-matrix.yml", opts)

		generateTask := runner.fetchTask(t, 10*time.Second)
		require.Equal(t, "generate", getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, generateTask.Id))
		runner.execTask(t, generateTask, &mockTaskOutcome{
			result:  runnerv1.Result_RESULT_SUCCESS,
			outputs: map[string]string{"matrix": "[1,2]"},
		})

		// The placeholder expands into one task per value, each named after its combination.
		seen := make([]string, 0, 2)
		for range 2 {
			task := runner.fetchTask(t, 10*time.Second)
			name := getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, task.Id)
			seen = append(seen, name)
			value := strings.TrimSuffix(strings.TrimPrefix(name, "build ("), ")")
			runner.execTask(t, task, &mockTaskOutcome{
				result:  runnerv1.Result_RESULT_SUCCESS,
				outputs: map[string]string{"result": "built-" + value},
			})
		}
		assert.ElementsMatch(t, []string{"build (1)", "build (2)"}, seen)

		reportTask := runner.fetchTask(t, 10*time.Second)
		require.Equal(t, "report", getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, reportTask.Id))
		buildNeed, ok := reportTask.Needs["build"]
		require.True(t, ok, "report must see the expanded combinations as its 'build' need")
		assert.Equal(t, runnerv1.Result_RESULT_SUCCESS, buildNeed.Result)
		assert.Contains(t, []string{"built-1", "built-2"}, buildNeed.Outputs["result"])
		runner.execTask(t, reportTask, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})

		// `if: false` is decided before the matrix is touched, so `gated` is skipped as one job.
		runner.fetchNoTask(t, 300*time.Millisecond)
	})
}
