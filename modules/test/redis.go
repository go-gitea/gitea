package test

import (
	"context"
	"os"
	"os/exec"
	"time"

	"gitea.dev/modules/nosql"
)

func waitRedisReady(t TestingT, conn string, dur time.Duration) (ready bool) {
	ctxTimed, cancel := context.WithTimeout(t.Context(), time.Second*5)
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

func redisServerCmd(t TestingT) *exec.Cmd {
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

func PrepareTestRedis(t TestingT) (string, func()) {
	redisConn := "redis://127.0.0.1:6379/0"
	var redisServer *exec.Cmd
	if !waitRedisReady(t, redisConn, 0) {
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
		if !waitRedisReady(t, redisConn, 5*time.Second) {
			t.Fatalf("failed to start redis-server")
		}
	}
	return redisConn, func() {
		if redisServer != nil {
			_ = redisServer.Process.Signal(os.Interrupt)
			_ = redisServer.Wait()
		}
	}
}
