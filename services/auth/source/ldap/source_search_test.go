// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ldap

import (
	"fmt"
	"net"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDial(t *testing.T) {
	silent, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer silent.Close()
	tlsServer := httptest.NewTLSServer(nil)
	defer tlsServer.Close()

	conn, err := dial(&Source{Host: " , ldap://127.0.0.1:1 , 127.0.0.1 , ", Port: int(netip.MustParseAddrPort(silent.Addr().String()).Port())})
	require.NoError(t, err)
	conn.Close()

	start := time.Now()
	conn, err = dial(&Source{Host: fmt.Sprintf("ldaps://%s, ldaps://%s", silent.Addr(), tlsServer.Listener.Addr()), SecurityProtocol: SecurityProtocolLDAPS, SkipVerify: true})
	require.NoError(t, err)
	assert.Less(t, time.Since(start), 10*time.Second)
	conn.Close()

	_, err = dial(&Source{Host: "ldaps://127.0.0.1:1, ldap://127.0.0.1", SecurityProtocol: SecurityProtocolStartTLS})
	assert.ErrorContains(t, err, "ldaps://127.0.0.1:1: ")
	assert.ErrorContains(t, err, "ldap://127.0.0.1: unsupported LDAP URL")

	_, err = dial(&Source{Host: " , "})
	assert.EqualError(t, err, "no LDAP host configured")
}
