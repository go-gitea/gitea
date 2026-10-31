// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitproxy

import (
	"bufio"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRelaySilentRemoteWhileClientOpen covers the uploadpack.keepAlive=0 scenario: after the
// request, the remote sends nothing for longer than the idle window while it builds the pack.
// The client side is still open, so the tunnel must survive — the read deadline only arms
// once the client has half-closed. Only after the half-close does a silent window reap it.
func TestRelaySilentRemoteWhileClientOpen(t *testing.T) {
	t.Parallel()
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	upstreamAddr := serveConns(t, func(conn net.Conn) {
		_, _ = bufio.NewReader(conn).ReadString('\n') // consume the request line
		time.Sleep(500 * time.Millisecond)            // then stay silent, longer than the idle window below
		_, _ = conn.Write([]byte("pack\n"))
		<-hold // then stay silent forever
	})

	client, git := connPair(t)
	upstream, err := net.Dial("tcp", upstreamAddr)
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		relay(client, upstream, 100*time.Millisecond)
		close(done)
	}()

	_, err = git.Write([]byte("request\n"))
	require.NoError(t, err)

	// client side still open: the remote's 500ms silence must not reap the tunnel
	_ = git.SetReadDeadline(time.Now().Add(5 * time.Second))
	reply, err := bufio.NewReader(git).ReadString('\n')
	require.NoError(t, err, "tunnel was reaped during upstream silence while the client was still connected")
	assert.Equal(t, "pack\n", reply)
	select {
	case <-done:
		t.Fatal("relay ended while the client was still connected")
	default:
	}

	// now the client half-closes and the remote stays silent: the idle window reaps the tunnel
	require.NoError(t, git.CloseWrite())
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("relay held the tunnel open after the client half-closed and the remote went silent")
	}
}
