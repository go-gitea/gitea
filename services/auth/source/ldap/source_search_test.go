// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ldap

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDialMultipleHostsFailover(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	validPort := tcpAddr.Port

	t.Run("SingleHost", func(t *testing.T) {
		source := &Source{
			Host:             "127.0.0.1",
			Port:             validPort,
			SecurityProtocol: SecurityProtocolUnencrypted,
		}
		conn, err := dial(source)
		require.NoError(t, err)
		require.NotNil(t, conn)
		conn.Close()
	})

	t.Run("FailoverToSecondHostWithPortOverride", func(t *testing.T) {
		source := &Source{
			Host:             fmt.Sprintf("127.0.0.1:1, 127.0.0.1:%d", validPort),
			Port:             1,
			SecurityProtocol: SecurityProtocolUnencrypted,
		}
		conn, err := dial(source)
		require.NoError(t, err)
		require.NotNil(t, conn)
		conn.Close()
	})

	t.Run("FailoverTrimsWhitespaceAndSkipsEmpty", func(t *testing.T) {
		source := &Source{
			Host:             " , 127.0.0.1:1 ,  127.0.0.1 , ",
			Port:             validPort,
			SecurityProtocol: SecurityProtocolUnencrypted,
		}
		conn, err := dial(source)
		require.NoError(t, err)
		require.NotNil(t, conn)
		conn.Close()
	})

	t.Run("AllHostsUnreachable", func(t *testing.T) {
		source := &Source{
			Host:             "127.0.0.1:1, 127.0.0.1:2",
			Port:             389,
			SecurityProtocol: SecurityProtocolUnencrypted,
		}
		conn, err := dial(source)
		assert.Nil(t, conn)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "127.0.0.1:1")
		assert.Contains(t, err.Error(), "127.0.0.1:2")
	})

	t.Run("NoHostConfigured", func(t *testing.T) {
		source := &Source{
			Host:             " , , ",
			Port:             389,
			SecurityProtocol: SecurityProtocolUnencrypted,
		}
		conn, err := dial(source)
		assert.Nil(t, conn)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no LDAP host configured")
	})
}
