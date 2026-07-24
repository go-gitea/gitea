// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/url"
	"testing"
	"time"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDynamicMatrixEvaluation is the integration test suite for the deferred dynamic matrix
// feature: jobs whose matrix references ${{ needs.*.outputs.* }} are stored as placeholders
// and re-expanded by ReEvaluateMatrixForJobWithNeeds once their dependency jobs complete.
func TestDynamicMatrixEvaluation(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-dynamic-matrix-eval", false)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, apiRepo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		t.Run("basic dynamic matrix from job outputs", func(t *testing.T) {
			const workflow = `name: basic-dynamic-matrix
on:
  push:
    paths: ['.gitea/workflows/basic-dynamic-matrix.yml']
jobs:
  generate:
    runs-on: ubuntu-latest
    outputs:
      matrix: ${{ steps.set.outputs.matrix }}
    steps:
      - id: set
        run: echo "matrix=[\"a\",\"b\",\"c\"]" >> "$GITHUB_OUTPUT"
  build:
    needs: [generate]
    runs-on: ubuntu-latest
    strategy:
      matrix:
        value: ${{ fromJson(needs.generate.outputs.matrix) }}
    steps:
      - run: echo "building ${{ matrix.value }}"
`
			opts := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, "create basic-dynamic-matrix.yml", workflow)
			createWorkflowFile(t, token, user2.Name, apiRepo.Name, ".gitea/workflows/basic-dynamic-matrix.yml", opts)

			generateTask := runner.fetchTask(t, 10*time.Second)
			assert.Equal(t, "generate", getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, generateTask.Id))
			runner.execTask(t, generateTask, &mockTaskOutcome{
				result:  runnerv1.Result_RESULT_SUCCESS,
				outputs: map[string]string{"matrix": `["a","b","c"]`},
			})

			seen := make(map[string]bool)
			for range 3 {
				task := runner.fetchTask(t, 10*time.Second)
				name := getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, task.Id)
				assert.Contains(t, []string{"build (a)", "build (b)", "build (c)"}, name)
				seen[name] = true
				runner.execTask(t, task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
			}
			assert.Len(t, seen, 3, "each of the 3 matrix values must produce a distinct task")
		})

		t.Run("multi-dimensional dynamic matrix", func(t *testing.T) {
			const workflow = `name: multi-dim-dynamic-matrix
on:
  push:
    paths: ['.gitea/workflows/multi-dim-dynamic-matrix.yml']
jobs:
  generate:
    runs-on: ubuntu-latest
    outputs:
      os: ${{ steps.set.outputs.os }}
      version: ${{ steps.set.outputs.version }}
    steps:
      - id: set
        run: |
          echo "os=[\"linux\",\"windows\"]" >> "$GITHUB_OUTPUT"
          echo "version=[1,2]" >> "$GITHUB_OUTPUT"
  build:
    needs: [generate]
    runs-on: ubuntu-latest
    strategy:
      matrix:
        os: ${{ fromJson(needs.generate.outputs.os) }}
        version: ${{ fromJson(needs.generate.outputs.version) }}
    steps:
      - run: echo "${{ matrix.os }} / ${{ matrix.version }}"
`
			opts := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, "create multi-dim-dynamic-matrix.yml", workflow)
			createWorkflowFile(t, token, user2.Name, apiRepo.Name, ".gitea/workflows/multi-dim-dynamic-matrix.yml", opts)

			generateTask := runner.fetchTask(t, 10*time.Second)
			assert.Equal(t, "generate", getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, generateTask.Id))
			runner.execTask(t, generateTask, &mockTaskOutcome{
				result: runnerv1.Result_RESULT_SUCCESS,
				outputs: map[string]string{
					"os":      `["linux","windows"]`,
					"version": `[1,2]`,
				},
			})

			expectedNames := map[string]bool{
				"build (linux, 1)":   false,
				"build (linux, 2)":   false,
				"build (windows, 1)": false,
				"build (windows, 2)": false,
			}
			for range 4 {
				task := runner.fetchTask(t, 10*time.Second)
				name := getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, task.Id)
				assert.Contains(t, expectedNames, name, "unexpected job name: %s", name)
				expectedNames[name] = true
				runner.execTask(t, task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
			}
			for name, seen := range expectedNames {
				assert.True(t, seen, "expected job %q was never dispatched", name)
			}
		})

		t.Run("empty matrix skips the job", func(t *testing.T) {
			const workflow = `name: empty-dynamic-matrix
on:
  push:
    paths: ['.gitea/workflows/empty-dynamic-matrix.yml']
jobs:
  generate:
    runs-on: ubuntu-latest
    outputs:
      matrix: ${{ steps.set.outputs.matrix }}
    steps:
      - id: set
        run: echo "matrix=[]" >> "$GITHUB_OUTPUT"
  build:
    needs: [generate]
    runs-on: ubuntu-latest
    strategy:
      matrix:
        value: ${{ fromJson(needs.generate.outputs.matrix) }}
    steps:
      - run: echo "${{ matrix.value }}"
`
			opts := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, "create empty-dynamic-matrix.yml", workflow)
			createWorkflowFile(t, token, user2.Name, apiRepo.Name, ".gitea/workflows/empty-dynamic-matrix.yml", opts)

			generateTask := runner.fetchTask(t, 10*time.Second)
			assert.Equal(t, "generate", getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, generateTask.Id))
			runner.execTask(t, generateTask, &mockTaskOutcome{
				result:  runnerv1.Result_RESULT_SUCCESS,
				outputs: map[string]string{"matrix": "[]"},
			})

			// An empty matrix expands to zero combinations, so the build job is skipped and no
			// task is ever dispatched (matching GitHub).
			runner.fetchNoTask(t, 2*time.Second)
		})

		t.Run("job-level if:false skips every combination", func(t *testing.T) {
			// Regression: the deferred-matrix job must still honour its job-level `if:`; expansion
			// must not bypass the if/concurrency gate and dispatch the combinations regardless.
			const workflow = `name: if-false-dynamic-matrix
on:
  push:
    paths: ['.gitea/workflows/if-false-dynamic-matrix.yml']
jobs:
  generate:
    runs-on: ubuntu-latest
    outputs:
      matrix: ${{ steps.set.outputs.matrix }}
    steps:
      - id: set
        run: echo "matrix=[\"a\",\"b\"]" >> "$GITHUB_OUTPUT"
  build:
    needs: [generate]
    if: false
    runs-on: ubuntu-latest
    strategy:
      matrix:
        value: ${{ fromJson(needs.generate.outputs.matrix) }}
    steps:
      - run: echo "building ${{ matrix.value }}"
`
			opts := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, "create if-false-dynamic-matrix.yml", workflow)
			createWorkflowFile(t, token, user2.Name, apiRepo.Name, ".gitea/workflows/if-false-dynamic-matrix.yml", opts)

			generateTask := runner.fetchTask(t, 10*time.Second)
			assert.Equal(t, "generate", getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, generateTask.Id))
			runner.execTask(t, generateTask, &mockTaskOutcome{
				result:  runnerv1.Result_RESULT_SUCCESS,
				outputs: map[string]string{"matrix": `["a","b"]`},
			})

			// `if: false` must skip the whole matrix: no build combination is ever dispatched.
			runner.fetchNoTask(t, 2*time.Second)
		})

		t.Run("downstream job depends on dynamic matrix", func(t *testing.T) {
			const workflow = `name: chained-dynamic-matrix
on:
  push:
    paths: ['.gitea/workflows/chained-dynamic-matrix.yml']
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
  report:
    needs: [build]
    runs-on: ubuntu-latest
    steps:
      - run: echo '${{ toJSON(needs.build.outputs) }}'
`
			opts := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, "create chained-dynamic-matrix.yml", workflow)
			createWorkflowFile(t, token, user2.Name, apiRepo.Name, ".gitea/workflows/chained-dynamic-matrix.yml", opts)

			generateTask := runner.fetchTask(t, 10*time.Second)
			assert.Equal(t, "generate", getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, generateTask.Id))
			runner.execTask(t, generateTask, &mockTaskOutcome{
				result:  runnerv1.Result_RESULT_SUCCESS,
				outputs: map[string]string{"matrix": "[1,2]"},
			})

			for range 2 {
				task := runner.fetchTask(t, 10*time.Second)
				name := getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, task.Id)
				assert.Contains(t, []string{"build (1)", "build (2)"}, name)
				value := "built-1"
				if name == "build (2)" {
					value = "built-2"
				}
				runner.execTask(t, task, &mockTaskOutcome{
					result:  runnerv1.Result_RESULT_SUCCESS,
					outputs: map[string]string{"result": value},
				})
			}

			reportTask := runner.fetchTask(t, 15*time.Second)
			assert.Equal(t, "report", getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, reportTask.Id))

			buildNeed, ok := reportTask.Needs["build"]
			require.True(t, ok, "report task must have 'build' in its needs")
			assert.Equal(t, runnerv1.Result_RESULT_SUCCESS, buildNeed.Result)

			assert.Contains(t, []string{"built-1", "built-2"}, buildNeed.Outputs["result"])

			runner.execTask(t, reportTask, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
		})
	})
}
