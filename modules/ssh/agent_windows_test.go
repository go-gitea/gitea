// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package ssh

import (
	"net"
	"testing"

	"github.com/Microsoft/go-winio"
	"github.com/stretchr/testify/require"
)

func dialTestAgent(t *testing.T, pipePath string) net.Conn {
	t.Helper()
	conn, err := winio.DialPipe(pipePath, nil)
	require.NoError(t, err)
	return conn
}
