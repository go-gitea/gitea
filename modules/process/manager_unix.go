// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

// SetSysProcAttribute sets the common SysProcAttrs for commands
func SetSysProcAttribute(cmd *exec.Cmd) {
	// When Gitea runs SubProcessA -> SubProcessB and SubProcessA gets killed by context timeout, use setpgid to make sure the sub processes can be reaped instead of leaving defunct(zombie) processes.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return KillCmd(cmd) }
}

// KillCmd kills the process group of cmd, ensuring grandchildren are reaped.
func KillCmd(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
