// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestActionsRunnerLongPollWake exercises the end-to-end glue from workflow
// dispatch to task delivery: a waiting FetchTask returns the queued task
// without waiting for the long-poll deadline. Wake channel mechanics are
// covered by unit tests in models/actions/tasks_version_notify_test.go.
func TestActionsRunnerLongPollWake(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
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

		defer test.MockVariableValue(&setting.Actions.RunnerLongPollTimeout, 5*time.Second)()

		var (
			wg      sync.WaitGroup
			task    *runnerv1.Task
			elapsed time.Duration
		)
		wg.Go(func() {
			start := time.Now()
			resp, err := runner.client.runnerServiceClient.FetchTask(t.Context(), connect.NewRequest(&runnerv1.FetchTaskRequest{
				TasksVersion: 0,
			}))
			elapsed = time.Since(start)
			assert.NoError(t, err)
			if resp != nil {
				task = resp.Msg.Task
			}
		})

		time.Sleep(100 * time.Millisecond)
		runURL := fmt.Sprintf("/%s/%s/actions/run?workflow=%s", user2.Name, repo.Name, "longpoll-wake.yml")
		session.MakeRequest(t, NewRequestWithValues(t, "POST", runURL, map[string]string{
			"ref": "refs/heads/" + repo.DefaultBranch,
		}), http.StatusSeeOther)

		wg.Wait()

		require.NotNil(t, task, "long-poll should return the queued task")
		require.Less(t, elapsed, 4*time.Second, "long-poll should return promptly after the wake, not wait the full timeout")
	})
}
