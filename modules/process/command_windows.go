// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package process

import "os/exec"

// There is no graceful way to kill a process on Windows at the moment

func setSysProcAttribute(cmd *exec.Cmd) {}

func (c *Cmd) onCancel() error {
	if c.onCancelUserFunc != nil {
		if err := c.onCancelUserFunc(); err != nil {
			return err
		}
	}
	return c.Process.Kill()
}
