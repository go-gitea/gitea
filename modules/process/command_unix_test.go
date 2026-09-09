// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build unix

package process

import (
	"bufio"
	"context"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandContextCancelKillProcessGroup(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// When executing external commands, there can be multiple subprocesses involved.
	// e.g.: Gitea ->  "git upload-pack" -> "git pack-objects".
	// If the context is canceled (HTTP client disconnects), all subprocesses must terminate.

	// Spawn a shell that itself spawns a long-lived background process and
	// prints its PID — mimicking git upload-pack spawning git pack-objects.

	r, w, err := os.Pipe()
	require.NoError(t, err)

	cmd := CommandContext(ctx, "sh", "-c", "sleep 600 & echo $!; wait")
	cmd.Stdout = w
	require.NoError(t, cmd.Start())
	_ = w.Close() // parent keeps only the read end

	t.Cleanup(func() {
		// make sure our test doesn't leave a zombie process even if test fails
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	})

	// Block until the shell prints the grandchild PID.
	scanner := bufio.NewScanner(r)
	require.True(t, scanner.Scan(), "expected grandchild PID on stdout")
	grandchildPID, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	require.NoError(t, err)
	_ = r.Close()

	// Sanity: grandchild must be alive before we cancel.
	grandchild, err := os.FindProcess(grandchildPID)
	require.NoError(t, err)
	require.NoError(t, grandchild.Signal(syscall.Signal(0)), "grandchild should be alive before cancel")

	// Cancel the context
	cancel()
	_ = cmd.Wait()

	// Subprocess should not exist after context cancel (killed by process group)
	assert.Eventually(t, func() bool {
		return grandchild.Signal(syscall.Signal(0)) != nil
	}, 5*time.Second, 10*time.Millisecond)
}
