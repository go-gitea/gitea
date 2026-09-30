// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	runnerv1 "gitea.dev/actionslib/runner/v1"
	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
      combos: ${{ steps.set.outputs.combos }}
    steps:
      - id: set
        run: |
          echo "matrix=[1,2]" >> "$GITHUB_OUTPUT"
          echo 'combos=[{"name":"x"},{"name":"y"}]' >> "$GITHUB_OUTPUT"
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
  static:
    needs: [generate]
    runs-on: ubuntu-latest
    strategy:
      matrix:
        os: [a, b]
    steps:
      - run: echo "${{ matrix.os }}"
  included:
    needs: [generate]
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include: ${{ fromJson(needs.generate.outputs.combos) }}
    steps:
      - run: echo "${{ matrix.name }}"
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
		_, _, run := getTaskAndJobAndRunByTaskID(t, generateTask.Id)
		runner.execTask(t, generateTask, &mockTaskOutcome{
			result:  runnerv1.Result_RESULT_SUCCESS,
			outputs: map[string]string{"matrix": "[1,2]", "combos": `[{"name":"x"},{"name":"y"}]`},
		})

		// execAttempt runs the attempt's remaining tasks, recording each job's name and AttemptJobID(order is not fixed).
		attemptJobIDs := map[string]int64{}
		var reportTask *runnerv1.Task
		execAttempt := func(t *testing.T, count int) []string {
			names := make([]string, 0, count)
			for range count {
				task := runner.fetchTask(t, 10*time.Second)
				_, taskJob, _ := getTaskAndJobAndRunByTaskID(t, task.Id)
				names = append(names, taskJob.Name)
				attemptJobIDs[taskJob.Name] = taskJob.AttemptJobID
				outcome := &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS}
				if value, ok := strings.CutPrefix(taskJob.Name, "build ("); ok {
					outcome.outputs = map[string]string{"result": "built-" + strings.TrimSuffix(value, ")")}
				}
				if taskJob.JobID == "report" {
					reportTask = task
				}
				runner.execTask(t, task, outcome)
			}
			runner.fetchNoTask(t, 300*time.Millisecond)
			return names
		}

		seen := execAttempt(t, 7)
		firstAttemptIDs := maps.Clone(attemptJobIDs)
		assert.ElementsMatch(t, []string{
			"build (1)", "build (2)", "included (x)", "included (y)", "static (a)", "static (b)", "report",
		}, seen)
		assert.Equal(t, actions_model.StatusSkipped, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: run.ID, Name: "gated"}).Status)

		buildNeed, ok := reportTask.Needs["build"]
		require.True(t, ok, "report must see the expanded combinations as its 'build' need")
		assert.Equal(t, runnerv1.Result_RESULT_SUCCESS, buildNeed.Result)
		assert.Contains(t, []string{"built-1", "built-2"}, buildNeed.Outputs["result"])

		// Re-running the whole run collapses the combinations back into a placeholder and re-derives
		// the matrix from the new attempt's outputs instead of reusing the previous combinations.
		req := NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/rerun", user2.Name, apiRepo.Name, run.ID))
		session.MakeRequest(t, req, http.StatusOK)

		generateTask2 := runner.fetchTask(t, 10*time.Second)
		require.Equal(t, "generate", getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, generateTask2.Id))
		assert.Equal(t, "2", generateTask2.Context.GetFields()["run_attempt"].GetStringValue())
		runner.execTask(t, generateTask2, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
			// `combos` is unchanged, so both `included` combinations recur and have to keep their AttemptJobIDs.
			outputs: map[string]string{"matrix": "[1,2,3]", "combos": `[{"name":"x"},{"name":"y"}]`},
		})

		// The matrix now yields a third value, while `static` is cloned as-is rather than collapsed.
		seenRerun := execAttempt(t, 8)
		assert.ElementsMatch(t, []string{
			"build (1)", "build (2)", "build (3)", "included (x)", "included (y)", "static (a)", "static (b)", "report",
		}, seenRerun)
		for name, firstID := range firstAttemptIDs {
			assert.Equal(t, firstID, attemptJobIDs[name], "%s keeps its AttemptJobID across attempts", name)
		}
	})
}
