// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sync"
	"testing"
	"time"

	runnerv1 "gitea.dev/actionslib/runner/v1"
	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsRunnerLongPollWake(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _url *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-longpoll-wake", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "longpoll-wake-runner", []string{"linux-runner"}, false)

		const wfFile = ".gitea/workflows/longpoll-wake.yml"
		const wfContent = `name: Long-poll Wake
on: workflow_dispatch
jobs:
  build:
    runs-on: linux-runner
    steps:
      - run: echo hi
`
		opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create "+wfFile, wfContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wfFile, opts)

		_, tasksVersion := runner.fetchTaskOnce(t, 0)
		defer test.MockVariableValue(&setting.Actions.RunnerLongPollTimeout, 5*time.Second)()
		t.Run("client deadline", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			resp, err := runner.client.runnerServiceClient.FetchTask(ctx, connect.NewRequest(&runnerv1.FetchTaskRequest{
				TasksVersion: tasksVersion,
			}))
			require.NoError(t, err)
			assert.Nil(t, resp.Msg.Task)
			assert.Equal(t, tasksVersion, resp.Msg.TasksVersion)
		})
		requestWritten := make(chan struct{})
		ctx := httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{
			WroteRequest: func(_info httptrace.WroteRequestInfo) { close(requestWritten) },
		})

		var (
			wg      sync.WaitGroup
			task    *runnerv1.Task
			elapsed time.Duration
		)
		wg.Go(func() {
			start := time.Now()
			resp, err := runner.client.runnerServiceClient.FetchTask(ctx, connect.NewRequest(&runnerv1.FetchTaskRequest{
				TasksVersion: tasksVersion,
			}))
			elapsed = time.Since(start)
			assert.NoError(t, err)
			if resp != nil {
				task = resp.Msg.Task
			}
		})

		<-requestWritten
		runURL := fmt.Sprintf("/%s/%s/actions/run?workflow=%s", user2.Name, repo.Name, "longpoll-wake.yml")
		session.MakeRequest(t, NewRequestWithValues(t, "POST", runURL, map[string]string{
			"ref": "refs/heads/" + repo.DefaultBranch,
		}), http.StatusSeeOther)

		wg.Wait()

		require.NotNil(t, task, "long-poll should return the queued task")
		require.Less(t, elapsed, 4*time.Second, "long-poll should return promptly after the wake, not wait the full timeout")
	})
}
