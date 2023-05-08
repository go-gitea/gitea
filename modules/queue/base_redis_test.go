// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/nosql"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func waitRedisReady(conn string, dur time.Duration) (ready bool) {
	ctxTimed, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	for t := time.Now(); ; time.Sleep(50 * time.Millisecond) {
		ret := nosql.GetManager().GetRedisClient(conn).Ping(ctxTimed)
		if ret.Err() == nil {
			return true
		}
		if time.Since(t) > dur {
			return false
		}
	}
}

func redisServerCmd(t *testing.T) *exec.Cmd {
	redisServerProg, err := exec.LookPath("redis-server")
	if err != nil {
		return nil
	}
	c := &exec.Cmd{
		Path:   redisServerProg,
		Args:   []string{redisServerProg, "--bind", "127.0.0.1", "--port", "6379"},
		Dir:    t.TempDir(),
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	return c
}

func TestBaseRedis(t *testing.T) {
	var redisServer *exec.Cmd
	defer func() {
		if redisServer != nil {
			_ = redisServer.Process.Signal(os.Interrupt)
			_ = redisServer.Wait()
		}
	}()
	if !waitRedisReady("redis://127.0.0.1:6379/0", 0) {
		redisServer = redisServerCmd(t)
		if redisServer == nil && os.Getenv("CI") != "" {
			t.Skip("redis-server not found")
			return
		}
		assert.NoError(t, redisServer.Start())
		if !assert.True(t, waitRedisReady("redis://127.0.0.1:6379/0", 5*time.Second), "start redis-server") {
			return
		}
	}

	testQueueBasic(t, newBaseRedisSimple, toBaseConfig("baseRedis", setting.QueueSettings{Length: 10}), false)
	testQueueBasic(t, newBaseRedisUnique, toBaseConfig("baseRedisUnique", setting.QueueSettings{Length: 10}), true)
}
