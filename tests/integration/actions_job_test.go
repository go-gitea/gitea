// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
)

func TestJobWithNeeds(t *testing.T) {
	testCases := []struct {
		treePath         string
		fileContent      string
		outcomes         map[string]*mockTaskOutcome
		expectedStatuses map[string]string
	}{
		{
			treePath: ".gitea/workflows/job-with-needs.yml",
			fileContent: `name: job-with-needs
on: 
  push:
    paths:
      - '.gitea/workflows/job-with-needs.yml'
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo job1
  job2:
    runs-on: ubuntu-latest
    needs: [job1]
    steps:
      - run: echo job2
`,
			outcomes: map[string]*mockTaskOutcome{
				"job1": {
					result: runnerv1.Result_RESULT_SUCCESS,
				},
				"job2": {
					result: runnerv1.Result_RESULT_SUCCESS,
				},
			},
			expectedStatuses: map[string]string{
				"job1": actions_model.StatusSuccess.String(),
				"job2": actions_model.StatusSuccess.String(),
			},
		},
		{
			treePath: ".gitea/workflows/job-with-needs-fail.yml",
			fileContent: `name: job-with-needs-fail
on: 
  push:
    paths:
      - '.gitea/workflows/job-with-needs-fail.yml'
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo job1
  job2:
    runs-on: ubuntu-latest
    needs: [job1]
    steps:
      - run: echo job2
`,
			outcomes: map[string]*mockTaskOutcome{
				"job1": {
					result: runnerv1.Result_RESULT_FAILURE,
				},
			},
			expectedStatuses: map[string]string{
				"job1": actions_model.StatusFailure.String(),
				"job2": actions_model.StatusSkipped.String(),
			},
		},
		{
			treePath: ".gitea/workflows/job-with-needs-fail-if.yml",
			fileContent: `name: job-with-needs-fail-if
on: 
  push:
    paths:
      - '.gitea/workflows/job-with-needs-fail-if.yml'
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo job1
  job2:
    runs-on: ubuntu-latest
    if: ${{ always() }}
    needs: [job1]
    steps:
      - run: echo job2
`,
			outcomes: map[string]*mockTaskOutcome{
				"job1": {
					result: runnerv1.Result_RESULT_FAILURE,
				},
				"job2": {
					result: runnerv1.Result_RESULT_SUCCESS,
				},
			},
			expectedStatuses: map[string]string{
				"job1": actions_model.StatusFailure.String(),
				"job2": actions_model.StatusSuccess.String(),
			},
		},
	}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-jobs-with-needs", false)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, apiRepo.Name, "mock-runner", []string{"ubuntu-latest"})

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("test %s", tc.treePath), func(t *testing.T) {
				// create the workflow file
				opts := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, fmt.Sprintf("create %s", tc.treePath), tc.fileContent)
				fileResp := createWorkflowFile(t, token, user2.Name, apiRepo.Name, tc.treePath, opts)

				// fetch and execute task
				for i := 0; i < len(tc.outcomes); i++ {
					task := runner.fetchTask(t)
					jobName := getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, task.Id)
					outcome := tc.outcomes[jobName]
					assert.NotNil(t, outcome)
					runner.execTask(t, task, outcome)
				}

				// check result
				req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/actions/tasks", user2.Name, apiRepo.Name)).
					AddTokenAuth(token)
				resp := MakeRequest(t, req, http.StatusOK)
				var actionTaskRespAfter api.ActionTaskResponse
				DecodeJSON(t, resp, &actionTaskRespAfter)
				for _, apiTask := range actionTaskRespAfter.Entries {
					if apiTask.HeadSHA != fileResp.Commit.SHA {
						continue
					}
					status := apiTask.Status
					assert.Equal(t, status, tc.expectedStatuses[apiTask.Name])
				}
			})
		}

		httpContext := NewAPITestContext(t, user2.Name, apiRepo.Name, auth_model.AccessTokenScopeWriteRepository)
		doAPIDeleteRepository(httpContext)(t)
	})
}

func TestJobNeedsMatrix(t *testing.T) {
	testCases := []struct {
		treePath          string
		fileContent       string
		outcomes          map[string]*mockTaskOutcome
		expectedTaskNeeds map[string]*runnerv1.TaskNeed // jobID => TaskNeed
	}{
		{
			treePath: ".gitea/workflows/jobs-outputs-with-matrix.yml",
			fileContent: `name: jobs-outputs-with-matrix
on: 
  push:
    paths:
      - '.gitea/workflows/jobs-outputs-with-matrix.yml'
jobs:
  job1:
    runs-on: ubuntu-latest
    outputs:
      output_1: ${{ steps.gen_output.outputs.output_1 }}
      output_2: ${{ steps.gen_output.outputs.output_2 }}
      output_3: ${{ steps.gen_output.outputs.output_3 }}
    strategy:
      matrix:
        version: [1, 2, 3]
    steps:
      - name: Generate output
        id: gen_output
        run: |
          version="${{ matrix.version }}"
          echo "output_${version}=${version}" >> "$GITHUB_OUTPUT"          
  job2:
    runs-on: ubuntu-latest
    needs: [job1]
    steps:
      - run: echo '${{ toJSON(needs.job1.outputs) }}'
`,
			outcomes: map[string]*mockTaskOutcome{
				"job1 (1)": {
					result: runnerv1.Result_RESULT_SUCCESS,
					outputs: map[string]string{
						"output_1": "1",
						"output_2": "",
						"output_3": "",
					},
				},
				"job1 (2)": {
					result: runnerv1.Result_RESULT_SUCCESS,
					outputs: map[string]string{
						"output_1": "",
						"output_2": "2",
						"output_3": "",
					},
				},
				"job1 (3)": {
					result: runnerv1.Result_RESULT_SUCCESS,
					outputs: map[string]string{
						"output_1": "",
						"output_2": "",
						"output_3": "3",
					},
				},
			},
			expectedTaskNeeds: map[string]*runnerv1.TaskNeed{
				"job1": {
					Result: runnerv1.Result_RESULT_SUCCESS,
					Outputs: map[string]string{
						"output_1": "1",
						"output_2": "2",
						"output_3": "3",
					},
				},
			},
		},
		{
			treePath: ".gitea/workflows/jobs-outputs-with-matrix-failure.yml",
			fileContent: `name: jobs-outputs-with-matrix-failure
on: 
  push:
    paths:
      - '.gitea/workflows/jobs-outputs-with-matrix-failure.yml'
jobs:
  job1:
    runs-on: ubuntu-latest
    outputs:
      output_1: ${{ steps.gen_output.outputs.output_1 }}
      output_2: ${{ steps.gen_output.outputs.output_2 }}
      output_3: ${{ steps.gen_output.outputs.output_3 }}
    strategy:
      matrix:
        version: [1, 2, 3]
    steps:
      - name: Generate output
        id: gen_output
        run: |
          version="${{ matrix.version }}"
          echo "output_${version}=${version}" >> "$GITHUB_OUTPUT"          
  job2:
    runs-on: ubuntu-latest
    if: ${{ always() }}
    needs: [job1]
    steps:
      - run: echo '${{ toJSON(needs.job1.outputs) }}'
`,
			outcomes: map[string]*mockTaskOutcome{
				"job1 (1)": {
					result: runnerv1.Result_RESULT_SUCCESS,
					outputs: map[string]string{
						"output_1": "1",
						"output_2": "",
						"output_3": "",
					},
				},
				"job1 (2)": {
					result: runnerv1.Result_RESULT_FAILURE,
					outputs: map[string]string{
						"output_1": "",
						"output_2": "",
						"output_3": "",
					},
				},
				"job1 (3)": {
					result: runnerv1.Result_RESULT_SUCCESS,
					outputs: map[string]string{
						"output_1": "",
						"output_2": "",
						"output_3": "3",
					},
				},
			},
			expectedTaskNeeds: map[string]*runnerv1.TaskNeed{
				"job1": {
					Result: runnerv1.Result_RESULT_FAILURE,
					Outputs: map[string]string{
						"output_1": "1",
						"output_2": "",
						"output_3": "3",
					},
				},
			},
		},
	}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-jobs-outputs-with-matrix", false)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, apiRepo.Name, "mock-runner", []string{"ubuntu-latest"})

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("test %s", tc.treePath), func(t *testing.T) {
				opts := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, fmt.Sprintf("create %s", tc.treePath), tc.fileContent)
				createWorkflowFile(t, token, user2.Name, apiRepo.Name, tc.treePath, opts)

				for i := 0; i < len(tc.outcomes); i++ {
					task := runner.fetchTask(t)
					jobName := getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, task.Id)
					outcome := tc.outcomes[jobName]
					assert.NotNil(t, outcome)
					runner.execTask(t, task, outcome)
				}

				task := runner.fetchTask(t)
				actualTaskNeeds := task.Needs
				assert.Len(t, actualTaskNeeds, len(tc.expectedTaskNeeds))
				for jobID, tn := range tc.expectedTaskNeeds {
					actualNeed := actualTaskNeeds[jobID]
					assert.Equal(t, tn.Result, actualNeed.Result)
					assert.Len(t, actualNeed.Outputs, len(tn.Outputs))
					for outputKey, outputValue := range tn.Outputs {
						assert.Equal(t, outputValue, actualNeed.Outputs[outputKey])
					}
				}
			})
		}

		httpContext := NewAPITestContext(t, user2.Name, apiRepo.Name, auth_model.AccessTokenScopeWriteRepository)
		doAPIDeleteRepository(httpContext)(t)
	})
}

func createActionsTestRepo(t *testing.T, authToken, repoName string, isPrivate bool) *api.Repository {
	req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{
		Name:          repoName,
		Private:       isPrivate,
		Readme:        "Default",
		AutoInit:      true,
		DefaultBranch: "main",
	}).AddTokenAuth(authToken)
	resp := MakeRequest(t, req, http.StatusCreated)
	var apiRepo api.Repository
	DecodeJSON(t, resp, &apiRepo)
	return &apiRepo
}

func getWorkflowCreateFileOptions(u *user_model.User, branch, msg, content string) *api.CreateFileOptions {
	return &api.CreateFileOptions{
		FileOptions: api.FileOptions{
			BranchName: branch,
			Message:    msg,
			Author: api.Identity{
				Name:  u.Name,
				Email: u.Email,
			},
			Committer: api.Identity{
				Name:  u.Name,
				Email: u.Email,
			},
			Dates: api.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		},
		ContentBase64: base64.StdEncoding.EncodeToString([]byte(content)),
	}
}

func createWorkflowFile(t *testing.T, authToken, ownerName, repoName, treePath string, opts *api.CreateFileOptions) *api.FileResponse {
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", ownerName, repoName, treePath), opts).
		AddTokenAuth(authToken)
	resp := MakeRequest(t, req, http.StatusCreated)
	var fileResponse api.FileResponse
	DecodeJSON(t, resp, &fileResponse)
	return &fileResponse
}

// getTaskJobNameByTaskID get the job name of the task by task ID
// there is currently not an API for querying a task by ID so we have to list all the tasks
func getTaskJobNameByTaskID(t *testing.T, authToken, ownerName, repoName string, taskID int64) string {
	// FIXME: we may need to query several pages
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/actions/tasks", ownerName, repoName)).
		AddTokenAuth(authToken)
	resp := MakeRequest(t, req, http.StatusOK)
	var taskRespBefore api.ActionTaskResponse
	DecodeJSON(t, resp, &taskRespBefore)
	for _, apiTask := range taskRespBefore.Entries {
		if apiTask.ID == taskID {
			return apiTask.Name
		}
	}
	return ""
}
