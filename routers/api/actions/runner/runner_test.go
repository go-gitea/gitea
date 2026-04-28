// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
)

type capabilityRegisterRequest struct {
	*runnerv1.RegisterRequest
	capabilities []string
}

func (r *capabilityRegisterRequest) GetCapabilities() []string {
	return r.capabilities
}

type capabilityDeclareRequest struct {
	*runnerv1.DeclareRequest
	capabilities []string
}

func (r *capabilityDeclareRequest) GetCapabilities() []string {
	return r.capabilities
}

func TestRunnerRequestHasCapabilityTypedAccessor(t *testing.T) {
	registerReq := &capabilityRegisterRequest{
		RegisterRequest: &runnerv1.RegisterRequest{},
		capabilities:    []string{"cancelling", "other"},
	}
	hasCapability, known := runnerRequestHasCapability(registerReq, "cancelling")
	assert.True(t, hasCapability)
	assert.True(t, known)

	declareReq := &capabilityDeclareRequest{
		DeclareRequest: &runnerv1.DeclareRequest{},
		capabilities:   nil,
	}
	hasCapability, known = runnerRequestHasCapability(declareReq, "cancelling")
	assert.False(t, hasCapability)
	assert.True(t, known)

	hasCapability, known = runnerRequestHasCapability(nil, "cancelling")
	assert.False(t, hasCapability)
	assert.False(t, known)
}

func TestApplyDeclareRequestToRunnerPreservesUnknownCapabilityState(t *testing.T) {
	runner := &actions_model.ActionRunner{
		HasCancellingSupport: true,
	}
	req := &runnerv1.DeclareRequest{
		Version: "1.2.3",
		Labels:  []string{"linux"},
	}

	cols := applyDeclareRequestToRunner(runner, req)
	assert.Equal(t, []string{"agent_labels", "version"}, cols)
	assert.True(t, runner.HasCancellingSupport)
	assert.Equal(t, "1.2.3", runner.Version)
	assert.Equal(t, []string{"linux"}, runner.AgentLabels)
}

func TestApplyDeclareRequestToRunnerUpdatesTypedCapabilityState(t *testing.T) {
	runner := &actions_model.ActionRunner{
		HasCancellingSupport: true,
	}
	req := &capabilityDeclareRequest{
		DeclareRequest: &runnerv1.DeclareRequest{
			Version: "1.2.3",
			Labels:  []string{"linux"},
		},
		capabilities: []string{},
	}

	cols := applyDeclareRequestToRunner(runner, req)
	assert.Equal(t, []string{"agent_labels", "version", "has_cancelling_support"}, cols)
	assert.False(t, runner.HasCancellingSupport)
}
