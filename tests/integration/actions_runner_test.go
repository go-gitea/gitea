// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	pingv1 "gitea.dev/actionslib/ping/v1"
	"gitea.dev/actionslib/ping/v1/pingv1connect"
	runnerv1 "gitea.dev/actionslib/runner/v1"
	"gitea.dev/actionslib/runner/v1/runnerv1connect"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/modules/setting"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockRunner struct {
	client *mockRunnerClient
}

type mockRunnerClient struct {
	pingServiceClient   pingv1connect.PingServiceClient
	runnerServiceClient runnerv1connect.RunnerServiceClient
}

func newMockRunner() *mockRunner {
	client := newMockRunnerClient("", "")
	return &mockRunner{client: client}
}

func newMockRunnerClient(uuid, token string) *mockRunnerClient {
	baseURL := setting.AppURL + "api/actions"

	client := connect.NewClient(connecthttp.NewTransport(http.DefaultClient, baseURL), func(next connect.ClientFunc) connect.ClientFunc {
		return func(ctx context.Context, spec connect.Spec) (connect.ClientStream, error) {
			info, _ := connect.CallInfoForClientContext(ctx)
			if uuid != "" {
				info.RequestHeader().Set("x-runner-uuid", uuid)
			}
			if token != "" {
				info.RequestHeader().Set("x-runner-token", token)
			}
			return next(ctx, spec)
		}
	})

	return &mockRunnerClient{
		pingServiceClient:   pingv1connect.NewPingServiceClient(client),
		runnerServiceClient: runnerv1connect.NewRunnerServiceClient(client),
	}
}

func (r *mockRunner) doPing(t *testing.T) {
	resp, err := r.client.pingServiceClient.Ping(t.Context(), &pingv1.PingRequest{
		Data: "mock-runner",
	})
	assert.NoError(t, err)
	assert.Equal(t, "Hello, mock-runner!", resp.Data)
}

func (r *mockRunner) doRegister(t *testing.T, name, token string, labels []string, ephemeral bool) {
	r.doPing(t)
	resp, err := r.client.runnerServiceClient.Register(t.Context(), &runnerv1.RegisterRequest{
		Name:      name,
		Token:     token,
		Version:   "mock-runner-version",
		Labels:    labels,
		Ephemeral: ephemeral,
	})
	assert.NoError(t, err)
	r.client = newMockRunnerClient(resp.Runner.Uuid, resp.Runner.Token)
}

func (r *mockRunner) registerAsRepoRunner(t *testing.T, ownerName, repoName, runnerName string, labels []string, ephemeral bool) {
	session := loginUser(t, ownerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/actions/runners/registration-token", ownerName, repoName)).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	registrationToken := DecodeJSON(t, resp, &struct {
		Token string `json:"token"`
	}{})
	r.doRegister(t, runnerName, registrationToken.Token, labels, ephemeral)
}

func (r *mockRunner) fetchTask(t *testing.T, timeout ...time.Duration) *runnerv1.Task {
	task := r.tryFetchTask(t, timeout...)
	require.NotNil(t, task, "failed to fetch a task")
	return task
}

func (r *mockRunner) fetchNoTask(t *testing.T) {
	task, _ := r.fetchTaskOnce(t, 0)
	require.Nil(t, task, "a task is fetched")
}

const defaultFetchTaskTimeout = 1 * time.Second

func (r *mockRunner) tryFetchTask(t *testing.T, timeout ...time.Duration) *runnerv1.Task {
	fetchTimeout := defaultFetchTaskTimeout
	if len(timeout) > 0 {
		fetchTimeout = timeout[0]
	}
	ddl := time.Now().Add(fetchTimeout)
	var task *runnerv1.Task
	for time.Now().Before(ddl) {
		task, _ = r.fetchTaskOnce(t, 0)
		if task != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	return task
}

// fetchTaskOnce performs a single FetchTask request with the given TasksVersion
// and returns the task (if any) and the TasksVersion from the response.
// Used to verify the production path where the runner sends the current version.
func (r *mockRunner) fetchTaskOnce(t *testing.T, tasksVersion int64) (*runnerv1.Task, int64) {
	resp, err := r.client.runnerServiceClient.FetchTask(t.Context(), &runnerv1.FetchTaskRequest{
		TasksVersion: tasksVersion,
	})
	require.NoError(t, err)
	return resp.Task, resp.TasksVersion
}

type mockTaskOutcome struct {
	result  runnerv1.Result
	outputs map[string]string
	logRows []*runnerv1.LogRow
}

func (r *mockRunner) execTask(t *testing.T, task *runnerv1.Task, outcome *mockTaskOutcome) {
	for idx, lr := range outcome.logRows {
		resp, err := r.client.runnerServiceClient.UpdateLog(t.Context(), &runnerv1.UpdateLogRequest{
			TaskId: task.Id,
			Index:  int64(idx),
			Rows:   []*runnerv1.LogRow{lr},
			NoMore: idx == len(outcome.logRows)-1,
		})
		assert.NoError(t, err)
		assert.EqualValues(t, idx+1, resp.AckIndex)
	}
	sentOutputKeys := make([]string, 0, len(outcome.outputs))
	for outputKey, outputValue := range outcome.outputs {
		resp, err := r.client.runnerServiceClient.UpdateTask(t.Context(), &runnerv1.UpdateTaskRequest{
			State: &runnerv1.TaskState{
				Id:     task.Id,
				Result: runnerv1.Result_RESULT_UNSPECIFIED,
			},
			Outputs: map[string]string{outputKey: outputValue},
		})
		assert.NoError(t, err)
		sentOutputKeys = append(sentOutputKeys, outputKey)
		assert.ElementsMatch(t, sentOutputKeys, resp.SentOutputs)
	}
	resp, err := r.client.runnerServiceClient.UpdateTask(t.Context(), &runnerv1.UpdateTaskRequest{
		State: &runnerv1.TaskState{
			Id:        task.Id,
			Result:    outcome.result,
			StoppedAt: timestamppb.Now(),
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, outcome.result, resp.State.Result)
}
