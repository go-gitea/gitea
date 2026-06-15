// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"testing"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"
	actions_model "gitea.dev/models/actions"

	"github.com/stretchr/testify/assert"
)

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
