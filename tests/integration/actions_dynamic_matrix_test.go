// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDynamicMatrix exercises the deferred-matrix expansion flow:
// job1 emits a JSON list as an output, job2's strategy.matrix references
// `${{ fromJSON(needs.job1.outputs.matrix) }}`. The server must wait for
// job1 to finish, then expand job2 into N concrete iterations — one per
// list entry — and dispatch a task per iteration.
//
// Regression for go-gitea/gitea#25179 / gitea/runner#393.
func TestDynamicMatrix(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-dynamic-matrix", false)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, apiRepo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		const treePath = ".gitea/workflows/dynamic-matrix.yml"
		const workflow = `name: dynamic-matrix
on:
  push:
    paths:
      - '.gitea/workflows/dynamic-matrix.yml'
jobs:
  job1:
    runs-on: ubuntu-latest
    outputs:
      matrix: ${{ steps.set.outputs.matrix }}
    steps:
      - id: set
        run: echo "matrix=[\"afile\",\"another file\"]" >> $GITHUB_OUTPUT
  job2:
    needs: job1
    runs-on: ubuntu-latest
    strategy:
      matrix:
        manifest: ${{ fromJSON(needs.job1.outputs.matrix) }}
    steps:
      - run: echo ${{ matrix.manifest }}
`
		opts := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, "create "+treePath, workflow)
		fileResp := createWorkflowFile(t, token, user2.Name, apiRepo.Name, treePath, opts)

		// 1. job1 fires first.
		task := runner.fetchTask(t)
		require.Equal(t, "job1", getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, task.Id))
		runner.execTask(t, task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
			outputs: map[string]string{
				"matrix": `["afile","another file"]`,
			},
		})

		// 2. Server expands the deferred matrix into two job2 iterations.
		// Each fetchTask should now return a distinct matrix iteration.
		seen := map[string]bool{}
		expectedNames := map[string]bool{
			"job2 (afile)":        true,
			"job2 (another file)": true,
		}
		for i := 0; i < 2; i++ {
			child := runner.fetchTask(t)
			name := getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, child.Id)
			assert.True(t, expectedNames[name], "unexpected child task name: %q", name)
			assert.False(t, seen[name], "matrix child %q dispatched twice", name)
			seen[name] = true

			// The runner must receive job1's outputs in task.Needs[job1].Outputs
			// so steps inside the matrix iteration can reference them via
			// `needs.job1.outputs.*` if they wish.
			require.Contains(t, child.Needs, "job1")
			assert.Equal(t, `["afile","another file"]`, child.Needs["job1"].Outputs["matrix"])

			runner.execTask(t, child, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
		}
		assert.Len(t, seen, 2, "both matrix iterations must be dispatched")

		// 3. No further tasks should be pending — the run has only 3 jobs total.
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/actions/tasks", user2.Name, apiRepo.Name)).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		taskResp := DecodeJSON(t, resp, &api.ActionTaskResponse{})
		var thisRunTasks []*api.ActionTask
		for _, apiTask := range taskResp.Entries {
			if apiTask.HeadSHA == fileResp.Commit.SHA {
				thisRunTasks = append(thisRunTasks, apiTask)
			}
		}
		require.Len(t, thisRunTasks, 3, "1 job1 + 2 job2 iterations")
		for _, apiTask := range thisRunTasks {
			assert.Equal(t, actions_model.StatusSuccess.String(), apiTask.Status, "task %q should be success", apiTask.Name)
		}

		// 4. No placeholder row left behind: every RunJob for this run
		// must carry a non-empty Name and have RawMatrix cleared on the
		// concrete iterations (placeholders would have RawMatrix set).
		jobs, err := unittest.GetXORMEngine().
			Table("action_run_job").
			Where("repo_id = ? AND commit_sha = ?", apiRepo.ID, fileResp.Commit.SHA).
			Asc("id").
			QueryString()
		require.NoError(t, err)
		require.Len(t, jobs, 3, "DB must contain exactly 3 RunJobs (no placeholder)")
		for _, j := range jobs {
			assert.Empty(t, j["raw_matrix"], "expanded children must not carry RawMatrix")
			assert.NotEmpty(t, j["name"], "every row needs a display name")
		}
	})
}
