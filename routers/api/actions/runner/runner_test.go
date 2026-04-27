// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"testing"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	actions_model "code.gitea.io/gitea/models/actions"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
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

func TestRunnerRequestHasCapabilityLegacyRegisterRequest(t *testing.T) {
	req := &runnerv1.RegisterRequest{}

	unknown := protowire.AppendTag(nil, 8, protowire.BytesType)
	unknown = protowire.AppendString(unknown, "cancelling")
	unknown = protowire.AppendTag(unknown, 8, protowire.BytesType)
	unknown = protowire.AppendString(unknown, "other")
	req.ProtoReflect().SetUnknown(unknown)

	hasCapability, known := runnerRequestHasCapability(req, "cancelling")
	assert.True(t, hasCapability)
	assert.True(t, known)

	hasCapability, known = runnerRequestHasCapability(req, "other")
	assert.True(t, hasCapability)
	assert.True(t, known)

	hasCapability, known = runnerRequestHasCapability(req, "missing")
	assert.False(t, hasCapability)
	assert.True(t, known)

	hasCapability, known = runnerRequestHasCapability(nil, "cancelling")
	assert.False(t, hasCapability)
	assert.False(t, known)

	cloned := proto.Clone(req).(*runnerv1.RegisterRequest)
	hasCapability, known = runnerRequestHasCapability(cloned, "cancelling")
	assert.True(t, hasCapability)
	assert.True(t, known)
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
}

func TestRunnerRequestHasCapabilityLegacyDeclareRequest(t *testing.T) {
	req := &runnerv1.DeclareRequest{}

	unknown := protowire.AppendTag(nil, 8, protowire.BytesType)
	unknown = protowire.AppendString(unknown, "cancelling")
	req.ProtoReflect().SetUnknown(unknown)

	hasCapability, known := runnerRequestHasCapability(req, "cancelling")
	assert.True(t, hasCapability)
	assert.True(t, known)
}

func TestApplyDeclareRequestToRunnerPreservesUnknownLegacyCapabilityState(t *testing.T) {
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

func TestApplyDeclareRequestToRunnerUpdatesKnownCapabilityState(t *testing.T) {
	runner := &actions_model.ActionRunner{}
	req := &runnerv1.DeclareRequest{
		Version: "1.2.3",
		Labels:  []string{"linux"},
	}

	unknown := protowire.AppendTag(nil, 8, protowire.BytesType)
	unknown = protowire.AppendString(unknown, "cancelling")
	req.ProtoReflect().SetUnknown(unknown)

	cols := applyDeclareRequestToRunner(runner, req)
	assert.Equal(t, []string{"agent_labels", "version", "has_cancelling_support"}, cols)
	assert.True(t, runner.HasCancellingSupport)
}

func TestApplyDeclareRequestToRunnerClearsTypedCapabilityState(t *testing.T) {
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
