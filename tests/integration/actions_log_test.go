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

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDownloadTaskLogs(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-download-task-logs", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"})

		treePath := ".gitea/workflows/download-task-logs.yml"
		fileContent := `name: download-task-logs
on: push
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo job1
`

		// create the workflow file
		opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", treePath), fileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, treePath, opts)

		now := time.Now()
		outcome := &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
			logRows: []*runnerv1.LogRow{
				{
					Time:    timestamppb.New(now),
					Content: "  \U0001F433  docker create image",
				},
				{
					Time:    timestamppb.New(now.Add(5 * time.Second)),
					Content: "job1",
				},
				{
					Time:    timestamppb.New(now.Add(8 * time.Second)),
					Content: "\U0001F3C1  Job succeeded",
				},
			},
		}

		// fetch and execute task
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
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%s/jobs/0/logs", user2.Name, repo.Name, runIndex)).
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

		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		doAPIDeleteRepository(httpContext)(t)
	})
}
