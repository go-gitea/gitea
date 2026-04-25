// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"testing"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/assert"
)

func TestRegisterRequestHasCapability(t *testing.T) {
	req := &runnerv1.RegisterRequest{}

	unknown := protowire.AppendTag(nil, 8, protowire.BytesType)
	unknown = protowire.AppendString(unknown, "cancelling")
	unknown = protowire.AppendTag(unknown, 8, protowire.BytesType)
	unknown = protowire.AppendString(unknown, "other")
	req.ProtoReflect().SetUnknown(unknown)

	assert.True(t, registerRequestHasCapability(req, "cancelling"))
	assert.True(t, registerRequestHasCapability(req, "other"))
	assert.False(t, registerRequestHasCapability(req, "missing"))
	assert.False(t, registerRequestHasCapability(nil, "cancelling"))

	cloned := proto.Clone(req).(*runnerv1.RegisterRequest)
	assert.True(t, registerRequestHasCapability(cloned, "cancelling"))
}
