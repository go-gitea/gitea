// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"testing"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"
	actions_model "gitea.dev/models/actions"

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

func TestRunnerRequestHasCancellingCapabilityTypedAccessor(t *testing.T) {
	registerReq := &capabilityRegisterRequest{
		RegisterRequest: &runnerv1.RegisterRequest{},
		capabilities:    []string{runnerCapabilityCancelling, "other"},
	}
	hasCapability, known := runnerRequestHasCancellingCapability(registerReq)
	assert.True(t, hasCapability)
	assert.True(t, known)

	declareReq := &capabilityDeclareRequest{
		DeclareRequest: &runnerv1.DeclareRequest{},
		capabilities:   nil,
	}
	hasCapability, known = runnerRequestHasCancellingCapability(declareReq)
	assert.False(t, hasCapability)
	assert.True(t, known)

	hasCapability, known = runnerRequestHasCancellingCapability(nil)
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
