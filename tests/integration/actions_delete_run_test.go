// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/routers/web/repo/actions"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestActionsDeleteRun(t *testing.T) {
	now := time.Now()
	testCase := struct {
		treePath         string
		fileContent      string
		outcomes         map[string]*mockTaskOutcome
		expectedStatuses map[string]string
	}{
		treePath: ".gitea/workflows/test1.yml",
		fileContent: `name: test1
on:
  push:
    paths:
      - .gitea/workflows/test1.yml
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo job1
  job2:
    runs-on: ubuntu-latest
    steps:
      - run: echo job2
  job3:
    runs-on: ubuntu-latest
    steps:
      - run: echo job3
`,
		outcomes: map[string]*mockTaskOutcome{
			"job1": {
				result: runnerv1.Result_RESULT_SUCCESS,
				logRows: []*runnerv1.LogRow{
					{
						Time:    timestamppb.New(now.Add(4 * time.Second)),
						Content: "  \U0001F433  docker create image",
					},
					{
						Time:    timestamppb.New(now.Add(5 * time.Second)),
						Content: "job1",
					},
					{
						Time:    timestamppb.New(now.Add(6 * time.Second)),
						Content: "\U0001F3C1  Job succeeded",
					},
				},
			},
			"job2": {
				result: runnerv1.Result_RESULT_SUCCESS,
				logRows: []*runnerv1.LogRow{
					{
						Time:    timestamppb.New(now.Add(4 * time.Second)),
						Content: "  \U0001F433  docker create image",
					},
					{
						Time:    timestamppb.New(now.Add(5 * time.Second)),
						Content: "job2",
					},
					{
						Time:    timestamppb.New(now.Add(6 * time.Second)),
						Content: "\U0001F3C1  Job succeeded",
					},
				},
			},
			"job3": {
				result: runnerv1.Result_RESULT_SUCCESS,
				logRows: []*runnerv1.LogRow{
					{
						Time:    timestamppb.New(now.Add(4 * time.Second)),
						Content: "  \U0001F433  docker create image",
					},
					{
						Time:    timestamppb.New(now.Add(5 * time.Second)),
						Content: "job3",
					},
					{
						Time:    timestamppb.New(now.Add(6 * time.Second)),
						Content: "\U0001F3C1  Job succeeded",
					},
				},
			},
		},
		expectedStatuses: map[string]string{
			"job1": actions_model.StatusSuccess.String(),
			"job2": actions_model.StatusSuccess.String(),
			"job3": actions_model.StatusSuccess.String(),
		},
	}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-delete-run-test", false)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, apiRepo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		opts := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, "create "+testCase.treePath, testCase.fileContent)
		createWorkflowFile(t, token, user2.Name, apiRepo.Name, testCase.treePath, opts)

		runIndex := ""
		for i := 0; i < len(testCase.outcomes); i++ {
			task := runner.fetchTask(t)
			jobName := getTaskJobNameByTaskID(t, token, user2.Name, apiRepo.Name, task.Id)
			outcome := testCase.outcomes[jobName]
			assert.NotNil(t, outcome)
			runner.execTask(t, task, outcome)
			runIndex = task.Context.GetFields()["run_number"].GetStringValue()
			assert.Equal(t, "1", runIndex)
		}

		for i := 0; i < len(testCase.outcomes); i++ {
			req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%s/jobs/%d", user2.Name, apiRepo.Name, runIndex, i), map[string]string{
				"_csrf": GetUserCSRFToken(t, session),
			})
			resp := session.MakeRequest(t, req, http.StatusOK)
			var listResp actions.ViewResponse
			err := json.Unmarshal(resp.Body.Bytes(), &listResp)
			assert.NoError(t, err)
			assert.Len(t, listResp.State.Run.Jobs, 3)

			req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%s/jobs/%d/logs", user2.Name, apiRepo.Name, runIndex, i)).
				AddTokenAuth(token)
			MakeRequest(t, req, http.StatusOK)
		}

		req := NewRequestWithValues(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%s", user2.Name, apiRepo.Name, runIndex), map[string]string{
			"_csrf": GetUserCSRFToken(t, session),
		})
		session.MakeRequest(t, req, http.StatusOK)

		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%s/delete", user2.Name, apiRepo.Name, runIndex), map[string]string{
			"_csrf": GetUserCSRFToken(t, session),
		})
		session.MakeRequest(t, req, http.StatusOK)

		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%s/delete", user2.Name, apiRepo.Name, runIndex), map[string]string{
			"_csrf": GetUserCSRFToken(t, session),
		})
		session.MakeRequest(t, req, http.StatusNotFound)

		req = NewRequestWithValues(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%s", user2.Name, apiRepo.Name, runIndex), map[string]string{
			"_csrf": GetUserCSRFToken(t, session),
		})
		session.MakeRequest(t, req, http.StatusNotFound)

		for i := 0; i < len(testCase.outcomes); i++ {
			req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%s/jobs/%d", user2.Name, apiRepo.Name, runIndex, i), map[string]string{
				"_csrf": GetUserCSRFToken(t, session),
			})
			session.MakeRequest(t, req, http.StatusNotFound)

			req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%s/jobs/%d/logs", user2.Name, apiRepo.Name, runIndex, i)).
				AddTokenAuth(token)
			MakeRequest(t, req, http.StatusNotFound)
		}
	})
}
