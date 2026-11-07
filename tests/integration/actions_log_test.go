// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	runnerv1 "gitea.dev/actionslib/runner/v1"
	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDownloadTaskLogs(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name        string
		logState    string
		logStart    time.Duration
		zstdEnabled bool
	}{
		{"download-task-logs-zstd", "enabled", time.Second, true},
		{"download-task-logs-no-zstd", "disabled", 4 * time.Second, false},
	}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-download-task-logs", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		for _, tc := range testCases {
			treePath := ".gitea/workflows/" + tc.name + ".yml"
			t.Run("test "+treePath, func(t *testing.T) {
				var resetFunc func()
				if tc.zstdEnabled {
					resetFunc = test.MockVariableValue(&setting.Actions.LogCompression, "zstd")
					assert.True(t, setting.Actions.LogCompression.IsZstd())
				} else {
					resetFunc = test.MockVariableValue(&setting.Actions.LogCompression, "none")
					assert.False(t, setting.Actions.LogCompression.IsZstd())
				}

				// create the workflow file
				opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create "+treePath, fmt.Sprintf(`name: %s
on:
  push:
    paths:
      - '%s'
jobs:
    job1:
      runs-on: ubuntu-latest
      steps:
        - run: echo job1 with zstd %s
    job2:
      runs-on: ubuntu-latest
      steps:
        - run: echo job2 with zstd %s
`, tc.name, treePath, tc.logState, tc.logState))
				createWorkflowFile(t, token, user2.Name, repo.Name, treePath, opts)

				// fetch and execute tasks
				for jobIndex := range 2 {
					outcome := &mockTaskOutcome{
						result: runnerv1.Result_RESULT_SUCCESS,
						logRows: []*runnerv1.LogRow{
							{Time: timestamppb.New(now.Add(tc.logStart)), Content: "  \U0001F433  docker create image"},
							{Time: timestamppb.New(now.Add(tc.logStart + time.Second)), Content: fmt.Sprintf("job%d zstd %s", jobIndex+1, tc.logState)},
							{Time: timestamppb.New(now.Add(tc.logStart + 2*time.Second)), Content: "\U0001F3C1  Job succeeded"},
						},
					}
					task := runner.fetchTask(t)
					runner.execTask(t, task, outcome)

					// check whether the log file exists
					logFileName := fmt.Sprintf("%s/%02x/%d.log", repo.FullName(), task.Id%256, task.Id)
					if setting.Actions.LogCompression.IsZstd() {
						logFileName += ".zst"
					}
					_, err := storage.Actions.Stat(logFileName)
					assert.NoError(t, err)

					_, job, run := getTaskAndJobAndRunByTaskID(t, task.Id)

					// download task logs and check content
					for _, logURL := range []string{
						fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/logs", user2.Name, repo.Name, run.ID, job.ID),
						// download task logs from API and check content
						fmt.Sprintf("/api/v1/repos/%s/%s/actions/jobs/%d/logs", user2.Name, repo.Name, job.ID),
					} {
						req := NewRequest(t, "GET", logURL).AddTokenAuth(token)
						resp := MakeRequest(t, req, http.StatusOK)
						logTextLines := strings.Split(strings.TrimSpace(resp.Body.String()), "\n")
						assert.Len(t, logTextLines, len(outcome.logRows))
						for idx, lr := range outcome.logRows {
							assert.Equal(
								t,
								fmt.Sprintf("%s %s", lr.Time.AsTime().Format("2006-01-02T15:04:05.0000000Z07:00"), lr.Content),
								logTextLines[idx],
							)
						}
					}
				}
				resetFunc()
			})
		}

		t.Run("DownloadRerunTaskLogs", func(t *testing.T) {
			treePath := ".gitea/workflows/download-rerun-logs.yml"
			fileContent := `name: download-rerun-logs
on:
  push:
    paths:
      - '.gitea/workflows/download-rerun-logs.yml'
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
`

			// create the workflow file
			opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create "+treePath, fileContent)
			createWorkflowFile(t, token, user2.Name, repo.Name, treePath, opts)

			// first run
			job1Task1 := runner.fetchTask(t)
			_, job1, _ := getTaskAndJobAndRunByTaskID(t, job1Task1.Id)
			runner.execTask(t, job1Task1, &mockTaskOutcome{
				result: runnerv1.Result_RESULT_SUCCESS,
				logRows: []*runnerv1.LogRow{
					{
						Time:    timestamppb.New(now.Add(1 * time.Second)),
						Content: "job1 first run",
					},
				},
			})
			job2Task1 := runner.fetchTask(t)
			_, job2, run := getTaskAndJobAndRunByTaskID(t, job2Task1.Id)
			runner.execTask(t, job2Task1, &mockTaskOutcome{
				result: runnerv1.Result_RESULT_SUCCESS,
				logRows: []*runnerv1.LogRow{
					{
						Time:    timestamppb.New(now.Add(1 * time.Second)),
						Content: "job2 first run",
					},
				},
			})

			// check job1 log
			req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/logs", user2.Name, repo.Name, run.ID, job1.ID)).
				AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			assert.Contains(t, resp.Body.String(), "job1 first run")
			// check job2 log
			req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/logs", user2.Name, repo.Name, run.ID, job2.ID)).
				AddTokenAuth(token)
			resp = MakeRequest(t, req, http.StatusOK)
			assert.Contains(t, resp.Body.String(), "job2 first run")

			// only rerun job2
			req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", user2.Name, repo.Name, run.ID, job2.ID))
			session.MakeRequest(t, req, http.StatusOK)
			job2TaskRerun := runner.fetchTask(t)
			runner.execTask(t, job2TaskRerun, &mockTaskOutcome{
				result: runnerv1.Result_RESULT_SUCCESS,
				logRows: []*runnerv1.LogRow{
					{
						Time:    timestamppb.New(now.Add(1 * time.Second)),
						Content: "job2 rerun",
					},
				},
			})

			run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
			job1Rerun := getLatestAttemptJobByTemplateJobID(t, run.ID, job1.ID)
			assert.Equal(t, run.LatestAttemptID, job1Rerun.RunAttemptID)
			job2Rerun := getLatestAttemptJobByTemplateJobID(t, run.ID, job2.ID)
			assert.Equal(t, run.LatestAttemptID, job2Rerun.RunAttemptID)

			// check job1 rerun log
			req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/logs", user2.Name, repo.Name, run.ID, job1Rerun.ID)).
				AddTokenAuth(token)
			resp = MakeRequest(t, req, http.StatusOK)
			assert.Contains(t, resp.Body.String(), "job1 first run") // should return the log of first run because job1 didn't rerun
			// check job2 rerun log
			req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/logs", user2.Name, repo.Name, run.ID, job2Rerun.ID)).
				AddTokenAuth(token)
			resp = MakeRequest(t, req, http.StatusOK)
			assert.Contains(t, resp.Body.String(), "job2 rerun")
		})
	})
}
