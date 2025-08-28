// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/test"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDownloadTaskLogs(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		treePath    string
		fileContent string
		outcome     []*mockTaskOutcome
		zstdEnabled bool
	}{
		{
			treePath: ".gitea/workflows/download-task-logs-zstd.yml",
			fileContent: `name: download-task-logs-zstd
on:
  push:
    paths:
      - '.gitea/workflows/download-task-logs-zstd.yml'
jobs:
    job1:
      runs-on: ubuntu-latest
      steps:
        - run: echo job1 with zstd enabled
    job2:
      runs-on: ubuntu-latest
      steps:
        - run: echo job2 with zstd enabled
`,
			outcome: []*mockTaskOutcome{
				{
					result: runnerv1.Result_RESULT_SUCCESS,
					logRows: []*runnerv1.LogRow{
						{
							Time:    timestamppb.New(now.Add(1 * time.Second)),
							Content: "  \U0001F433  docker create image",
						},
						{
							Time:    timestamppb.New(now.Add(2 * time.Second)),
							Content: "job1 zstd enabled",
						},
						{
							Time:    timestamppb.New(now.Add(3 * time.Second)),
							Content: "\U0001F3C1  Job succeeded",
						},
					},
				},
				{
					result: runnerv1.Result_RESULT_SUCCESS,
					logRows: []*runnerv1.LogRow{
						{
							Time:    timestamppb.New(now.Add(1 * time.Second)),
							Content: "  \U0001F433  docker create image",
						},
						{
							Time:    timestamppb.New(now.Add(2 * time.Second)),
							Content: "job2 zstd enabled",
						},
						{
							Time:    timestamppb.New(now.Add(3 * time.Second)),
							Content: "\U0001F3C1  Job succeeded",
						},
					},
				},
			},
			zstdEnabled: true,
		},
		{
			treePath: ".gitea/workflows/download-task-logs-no-zstd.yml",
			fileContent: `name: download-task-logs-no-zstd
on:
  push:
    paths:
      - '.gitea/workflows/download-task-logs-no-zstd.yml'
jobs:
    job1:
      runs-on: ubuntu-latest
      steps:
        - run: echo job1 with zstd disabled
    job2:
      runs-on: ubuntu-latest
      steps:
        - run: echo job2 with zstd disabled
`,
			outcome: []*mockTaskOutcome{
				{
					result: runnerv1.Result_RESULT_SUCCESS,
					logRows: []*runnerv1.LogRow{
						{
							Time:    timestamppb.New(now.Add(4 * time.Second)),
							Content: "  \U0001F433  docker create image",
						},
						{
							Time:    timestamppb.New(now.Add(5 * time.Second)),
							Content: "job1 zstd disabled",
						},
						{
							Time:    timestamppb.New(now.Add(6 * time.Second)),
							Content: "\U0001F3C1  Job succeeded",
						},
					},
				},
				{
					result: runnerv1.Result_RESULT_SUCCESS,
					logRows: []*runnerv1.LogRow{
						{
							Time:    timestamppb.New(now.Add(4 * time.Second)),
							Content: "  \U0001F433  docker create image",
						},
						{
							Time:    timestamppb.New(now.Add(5 * time.Second)),
							Content: "job2 zstd disabled",
						},
						{
							Time:    timestamppb.New(now.Add(6 * time.Second)),
							Content: "\U0001F3C1  Job succeeded",
						},
					},
				},
			},
			zstdEnabled: false,
		},
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
			t.Run("test "+tc.treePath, func(t *testing.T) {
				var resetFunc func()
				if tc.zstdEnabled {
					resetFunc = test.MockVariableValue(&setting.Actions.LogCompression, "zstd")
					assert.True(t, setting.Actions.LogCompression.IsZstd())
				} else {
					resetFunc = test.MockVariableValue(&setting.Actions.LogCompression, "none")
					assert.False(t, setting.Actions.LogCompression.IsZstd())
				}

				// create the workflow file
				opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create "+tc.treePath, tc.fileContent)
				createWorkflowFile(t, token, user2.Name, repo.Name, tc.treePath, opts)

				// fetch and execute tasks
				for jobIndex, outcome := range tc.outcome {
					task := runner.fetchTask(t)
					runner.execTask(t, task, outcome)

					// check whether the log file exists
					logFileName := fmt.Sprintf("%s/%02x/%d.log", repo.FullName(), task.Id%256, task.Id)
					if setting.Actions.LogCompression.IsZstd() {
						logFileName += ".zst"
					}
					_, err := storage.Actions.Stat(logFileName)
					assert.NoError(t, err)

					// download task logs and check content
					runIndex := task.Context.GetFields()["run_number"].GetStringValue()
					req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%s/jobs/%d/logs", user2.Name, repo.Name, runIndex, jobIndex)).
						AddTokenAuth(token)
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

					runID, _ := strconv.ParseInt(task.Context.GetFields()["run_id"].GetStringValue(), 10, 64)

					jobs, err := actions_model.GetRunJobsByRunID(t.Context(), runID)
					assert.NoError(t, err)
					assert.Len(t, jobs, len(tc.outcome))
					jobID := jobs[jobIndex].ID

					// download task logs from API and check content
					req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/actions/jobs/%d/logs", user2.Name, repo.Name, jobID)).
						AddTokenAuth(token)
					resp = MakeRequest(t, req, http.StatusOK)
					logTextLines = strings.Split(strings.TrimSpace(resp.Body.String()), "\n")
					assert.Len(t, logTextLines, len(outcome.logRows))
					for idx, lr := range outcome.logRows {
						assert.Equal(
							t,
							fmt.Sprintf("%s %s", lr.Time.AsTime().Format("2006-01-02T15:04:05.0000000Z07:00"), lr.Content),
							logTextLines[idx],
						)
					}
				}
				resetFunc()
			})
		}
	})
}
