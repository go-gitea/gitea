// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build unix

package process

import (
	"os/exec"
	"syscall"

	"gitea.dev/modules/util"
)

func setSysProcAttribute(cmd *exec.Cmd) {
	// When Gitea runs SubProcessA -> SubProcessB and SubProcessA gets killed by context cancel,
	// use process group to make sure the sub processes can be killed and reaped instead of leaving defunct(zombie) processes.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func (c *Cmd) onCancel() error {
	if c.onCancelUserFunc != nil {
		if err := c.onCancelUserFunc(); err != nil {
			return err
		}
	}
	sig := util.Iif(c.termGraceful, syscall.SIGTERM, syscall.SIGKILL)
	// kill the whole process group
	// ATTENTION: do not access PID after Wait or in other goroutine, it will just cause PID reuse data-race.
	// There is no easy solution to implement "first SIGTERM then SIGKILL" in a safe way, only one signal can be sent to the process group.
	return syscall.Kill(-c.Process.Pid, sig)
}
