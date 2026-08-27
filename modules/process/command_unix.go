// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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
	return syscall.Kill(-c.Process.Pid, sig)
}
