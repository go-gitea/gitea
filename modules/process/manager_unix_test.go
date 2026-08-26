// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package process

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetSysProcAttributeKillsProcessGroup verifies that cancelling a context
// kills not only the direct child but also grandchildren it spawned.
//
// This mirrors the git upload-pack -> git pack-objects relationship during a
// git clone: if an HTTP client disconnects mid-transfer, all subprocesses must
// die. exec.CommandContext sends SIGKILL to the direct PID only; because
// SetSysProcAttribute sets Setpgid:true the grandchild is in the same process
// group but is NOT killed, leaking it until it finishes on its own.
func TestSetSysProcAttributeKillsProcessGroup(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Spawn a shell that itself spawns a long-lived background process and
	// prints its PID — mimicking git upload-pack spawning git pack-objects.
	r, w, err := os.Pipe()
	require.NoError(t, err)

	cmd := exec.CommandContext(ctx, "sh", "-c", "sleep 600 & echo $!; wait")
	cmd.Stdout = w
	SetSysProcAttribute(cmd)
	require.NoError(t, cmd.Start())
	w.Close() // parent keeps only the read end

	// Always kill the process group on test exit so a failing assertion does
	// not leak the grandchild (sleep 600) as an orphan. Setpgid:true makes
	// the shell's PGID equal its own PID, so -cmd.Process.Pid targets the group.
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	})

	// Block until the shell prints the grandchild PID.
	scanner := bufio.NewScanner(r)
	require.True(t, scanner.Scan(), "expected grandchild PID on stdout")
	grandchildPID, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	require.NoError(t, err)
	r.Close()

	// Sanity: grandchild must be alive before we cancel.
	grandchild, err := os.FindProcess(grandchildPID)
	require.NoError(t, err)
	require.NoError(t, grandchild.Signal(syscall.Signal(0)), "grandchild should be alive before cancel")

	// Cancel the context — exec.CommandContext should propagate the kill to the
	// whole process group, not just the direct child (the shell).
	cancel()
	_ = cmd.Wait()

	// Poll until the grandchild is gone or the deadline is exceeded.
	// Signal(0) returns ESRCH (non-nil) once the process no longer exists.
	assert.Eventually(t, func() bool {
		return grandchild.Signal(syscall.Signal(0)) != nil
	}, 5*time.Second, 10*time.Millisecond,
		"grandchild process %d is still running after context cancel — process group was not killed (git pack-objects leak)", grandchildPID)
}
