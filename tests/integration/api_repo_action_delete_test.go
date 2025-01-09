// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"

	"github.com/stretchr/testify/assert"
)

func TestRepoActionDelete(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

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
		// check result
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions", httpContext.Username, httpContext.Reponame))
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		var runIDs []int64
		list := htmlDoc.doc.Find("#action-actions input.action-checkbox:not(:disabled)")
		list.Each(func(i int, s *goquery.Selection) {
			idStr, exists := s.Attr("data-action-id")
			if exists {
				runID, err := strconv.ParseInt(idStr, 10, 64)
				assert.NoError(t, err)
				runIDs = append(runIDs, runID)
			}
		})

		assert.NotEmpty(t, runIDs)
		csrf := GetUserCSRFToken(t, session)
		reqDelete := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/%s/%s/actions/runs/delete", httpContext.Username, httpContext.Reponame), map[string]interface{}{
			"actionIds": runIDs,
			"_csrf":     csrf,
		}).AddTokenAuth(token).
			SetHeader("X-Csrf-Token", csrf)
		session.MakeRequest(t, reqDelete, http.StatusNoContent)

		// should not found
		_, err := actions_model.GetRunsByIDsAndTriggerUserID(context.Background(), runIDs, user2.ID)
		assert.EqualError(t, err, fmt.Errorf("run with ids %d: %w", runIDs, util.ErrNotExist).Error())
		doAPIDeleteRepository(httpContext)
	})
}
