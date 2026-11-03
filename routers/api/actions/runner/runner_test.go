// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"errors"
	"net/http/httptest"
	"testing"

	runnerv1 "gitea.dev/actionslib/runner/v1"
	"gitea.dev/actionslib/runner/v1/runnerv1connect"
	actions_model "gitea.dev/models/actions"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlainErrorReachesRunnerAsUnknownWithMessage(t *testing.T) {
	_, handler := NewRunnerServiceHandler()
	server := httptest.NewServer(handler)
	defer server.Close()

	client := runnerv1connect.NewRunnerServiceClient(connect.NewClient(connecthttp.NewTransport(server.Client(), server.URL)))
	_, err := client.Register(t.Context(), &runnerv1.RegisterRequest{})
	connectErr, ok := errors.AsType[*connect.Error](err)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnknown, connectErr.Code())
	assert.Equal(t, "missing runner token, name", connectErr.Message())
}

func TestApplyDeclareRequestToRunnerAdvertisedCapabilityEnablesCancelling(t *testing.T) {
	runner := &actions_model.ActionRunner{}
	req := &runnerv1.DeclareRequest{
		Version:      "1.2.3",
		Labels:       []string{"linux"},
		Capabilities: []string{runnerCapabilityCancelling, "other"},
	}

	cols := applyDeclareRequestToRunner(runner, req)
	assert.Equal(t, []string{"agent_labels", "version", "has_cancelling_support"}, cols)
	assert.True(t, runner.HasCancellingSupport)
	assert.Equal(t, "1.2.3", runner.Version)
	assert.Equal(t, []string{"linux"}, runner.AgentLabels)
}

func TestApplyDeclareRequestToRunnerMissingCapabilityDisablesCancelling(t *testing.T) {
	runner := &actions_model.ActionRunner{
		HasCancellingSupport: true,
	}
	req := &runnerv1.DeclareRequest{
		Version: "1.2.3",
		Labels:  []string{"linux"},
	}

	cols := applyDeclareRequestToRunner(runner, req)
	assert.Equal(t, []string{"agent_labels", "version", "has_cancelling_support"}, cols)
	assert.False(t, runner.HasCancellingSupport)
}

func TestApplyDeclareRequestToRunnerUnchangedCapabilityOmitsColumn(t *testing.T) {
	runner := &actions_model.ActionRunner{
		HasCancellingSupport: true,
	}
	req := &runnerv1.DeclareRequest{
		Version:      "1.2.3",
		Labels:       []string{"linux"},
		Capabilities: []string{runnerCapabilityCancelling},
	}

	cols := applyDeclareRequestToRunner(runner, req)
	assert.Equal(t, []string{"agent_labels", "version"}, cols)
	assert.True(t, runner.HasCancellingSupport)
}
