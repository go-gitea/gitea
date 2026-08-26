// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package process

import (
	"os/exec"
)

// SetSysProcAttribute sets the common SysProcAttrs for commands
func SetSysProcAttribute(cmd *exec.Cmd) {
	// Do nothing
}

// KillCmd kills the process; on Windows there are no process groups.
func KillCmd(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
