// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"net"
	"os"
	"os/exec"
	"time"
)

const (
	testRedisHost    = "127.0.0.1"
	testRedisPort    = "6379"
	testRedisAddr    = testRedisHost + ":" + testRedisPort
	testRedisConnStr = "redis://" + testRedisAddr + "/0"
)

// waitRedisReady reports whether redis accepts connections within dur. Redis
// binds its listener last during startup, so a successful dial means it can
// serve. A plain dial, not a redis PING: the client retries its pool on a
// refused connect, which makes the "is one already running" probe take ~1s.
func waitRedisReady(dur time.Duration) bool {
	for start := time.Now(); ; time.Sleep(50 * time.Millisecond) {
		conn, err := net.DialTimeout("tcp", testRedisAddr, time.Second)
		if err == nil {
			_ = conn.Close()
			return true
		}
		if time.Since(start) > dur {
			return false
		}
	}
}

func redisServerCmd(t TestingT) *exec.Cmd {
	redisServerProg, err := exec.LookPath("redis-server")
	if err != nil {
		return nil
	}
	return &exec.Cmd{
		Path:   redisServerProg,
		Args:   []string{redisServerProg, "--bind", testRedisHost, "--port", testRedisPort},
		Dir:    t.TempDir(),
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

func PrepareTestRedis(t TestingT) (string, func()) {
	var redisServer *exec.Cmd
	if !waitRedisReady(0) {
		redisServer = redisServerCmd(t)
		if redisServer == nil {
			if AllowSkipExternalService() {
				t.Skipf("redis-server command not found, skipped")
			} else {
				t.Fatalf("no redis server or command, but skipping is not allowed")
			}
		}
		if err := redisServer.Start(); err != nil {
			t.Fatalf("failed to start redis-server: %v", err)
		}
		if !waitRedisReady(5 * time.Second) {
			t.Fatalf("failed to start redis-server")
		}
	}
	return testRedisConnStr, func() {
		if redisServer != nil {
			_ = redisServer.Process.Signal(os.Interrupt)
			_ = redisServer.Wait()
		}
	}
}
