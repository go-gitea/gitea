// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const testRedisAddr = "127.0.0.1:6379"

// waitRedisReady reports whether redis accepts connections on addr within dur.
// Redis binds its listener last during startup, so a successful dial means it
// can serve. A plain dial, not a redis PING: the client retries its pool on a
// refused connect, which makes the "is one already running" probe take ~1s.
func waitRedisReady(network, addr string, dur time.Duration) bool {
	for start := time.Now(); ; time.Sleep(5 * time.Millisecond) {
		conn, err := net.DialTimeout(network, addr, time.Second)
		if err == nil {
			_ = conn.Close()
			return true
		}
		if time.Since(start) > dur {
			return false
		}
	}
}

// PrepareTestRedis returns a connection string to a running redis, reusing one
// already listening on the well-known port, otherwise starting one for the
// duration of the test.
func PrepareTestRedis(t TestingT) string {
	if waitRedisReady("tcp", testRedisAddr, 0) {
		return "redis://" + testRedisAddr + "/0"
	}
	redisServerProg, err := exec.LookPath("redis-server")
	if err != nil {
		if AllowSkipExternalService() {
			t.Skipf("redis-server command not found, skipped")
		} else {
			t.Fatalf("no redis server or command, but skipping is not allowed")
		}
		return ""
	}
	// listen on a socket of our own rather than a port, so that packages running
	// in parallel can neither reach nor tear down each other's server
	dir := t.TempDir()
	socket := filepath.Join(dir, "redis.sock")
	redisServer := &exec.Cmd{
		Path:   redisServerProg,
		Args:   []string{redisServerProg, "--port", "0", "--unixsocket", socket},
		Dir:    dir,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	if err := redisServer.Start(); err != nil {
		t.Fatalf("failed to start redis-server: %v", err)
	}
	t.Cleanup(func() {
		_ = redisServer.Process.Signal(os.Interrupt)
		_ = redisServer.Wait()
	})
	if !waitRedisReady("unix", socket, 5*time.Second) {
		t.Fatalf("failed to start redis-server")
	}
	return "redis+socket://" + socket
}
