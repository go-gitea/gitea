// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
)

func TestUnregisteredRunnerErrorIsConnectUnauthenticated(t *testing.T) {
	err := unregisteredRunnerError()
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	var connectErr *connect.Error
	assert.ErrorAs(t, err, &connectErr)
	assert.Equal(t, "unregistered runner", connectErr.Message())
}
