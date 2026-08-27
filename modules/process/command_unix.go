// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build unix

package process

import (
	"os/exec"
	"syscall"
	"time"

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
	if sig == syscall.SIGTERM && c.Cmd.WaitDelay > 0 {
		delay := c.Cmd.WaitDelay
		go func() {
			time.Sleep(delay)
			_ = c.signalProcessGroup(syscall.SIGKILL)
		}()
	}
	return c.signalProcessGroup(sig)
}

// signalProcessGroup sends sig to the process group, skipping if the process has already been reaped.
func (c *Cmd) signalProcessGroup(sig syscall.Signal) error {
	if c.Cmd.ProcessState != nil {
		return nil
	}
	return syscall.Kill(-c.Process.Pid, sig)
}
