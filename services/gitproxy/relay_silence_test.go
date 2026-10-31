// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitproxy

import (
	"bufio"
	"net"
	"testing"
	"time"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

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

// TestDirectIdleOverGitTimeout: the direct-tunnel reap bound must exceed every
// [git.timeout], so it can only fire after git itself was killed — protocol v2
// half-closes before receiving the pack (fetch-pack.c close(fd[1])), and a live
// clone may be silent for a long pack build.
func TestDirectIdleOverGitTimeout(t *testing.T) {
	defer test.MockVariableValue(&setting.Git.Timeout.Migrate, 600)()
	defer test.MockVariableValue(&setting.Git.Timeout.Mirror, 300)()
	defer test.MockVariableValue(&setting.Git.Timeout.GC, 60)()
	assert.Greater(t, directIdle(), 600*time.Second)

	defer test.MockVariableValue(&setting.Git.Timeout.Migrate, 5)()
	defer test.MockVariableValue(&setting.Git.Timeout.Mirror, 4)()
	defer test.MockVariableValue(&setting.Git.Timeout.GC, 3)()
	assert.Equal(t, 35*time.Second, directIdle())
}

// TestRelayV2HalfCloseSlowPack: a half-closed client (protocol v2 close(fd[1]))
// waiting out a silent pack build must get the pack as long as the silence is
// within the idle bound; only a silent window past the bound reaps the tunnel.
func TestRelayV2HalfCloseSlowPack(t *testing.T) {
	t.Parallel()
	slowPack := func(silence time.Duration) (client net.Conn, git *net.TCPConn, done chan struct{}) {
		upstreamAddr := serveConns(t, func(conn net.Conn) {
			_, _ = bufio.NewReader(conn).ReadString('\n') // the fetch request
			time.Sleep(silence)                           // silent pack build, keepAlive=0
			_, _ = conn.Write([]byte("PACK"))
		})
		client, git = connPair(t)
		upstream, err := net.Dial("tcp", upstreamAddr)
		require.NoError(t, err)
		done = make(chan struct{})
		go func() {
			relay(client, upstream, time.Hour) // directIdle() is hours, not 30s
			close(done)
		}()
		_, err = git.Write([]byte("fetch-request\n"))
		require.NoError(t, err)
		require.NoError(t, git.CloseWrite()) // protocol v2 half-close before the pack
		return client, git, done
	}

	t.Run("PackWithinIdleDelivered", func(t *testing.T) {
		t.Parallel()
		_, git, done := slowPack(300 * time.Millisecond)
		_ = git.SetReadDeadline(time.Now().Add(5 * time.Second))
		buf := make([]byte, 4)
		n, err := git.Read(buf)
		require.NoError(t, err, "tunnel was reaped during a silent pack build within the idle bound")
		assert.Equal(t, "PACK", string(buf[:n]))
		<-done
	})

	t.Run("SilentPastIdleReaped", func(t *testing.T) {
		t.Parallel()
		upstreamAddr := serveConns(t, func(conn net.Conn) {
			_, _ = bufio.NewReader(conn).ReadString('\n')
			<-make(chan struct{}) // silent forever
		})
		client, git := connPair(t)
		upstream, err := net.Dial("tcp", upstreamAddr)
		require.NoError(t, err)
		done := make(chan struct{})
		go func() {
			relay(client, upstream, 100*time.Millisecond)
			close(done)
		}()
		_, err = git.Write([]byte("fetch-request\n"))
		require.NoError(t, err)
		require.NoError(t, git.CloseWrite())
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("tunnel held open past the idle bound after the half-close")
		}
	})
}
