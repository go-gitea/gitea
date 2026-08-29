// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package process

import (
	"bytes"
	"context"
	"os/exec"
)

type Cmd struct {
	*exec.Cmd

	onCancelUserFunc func() error
	termGraceful     bool
}

func (c *Cmd) WithOnCancelGracefully(userFunc func() error) *Cmd {
	c.termGraceful, c.onCancelUserFunc = true, userFunc
	return c
}

func (c *Cmd) WithOnCancelForceKill(userFunc func() error) *Cmd {
	c.termGraceful, c.onCancelUserFunc = false, userFunc
	return c
}

func (c *Cmd) WithDir(dir string) *Cmd {
	c.Cmd.Dir = dir
	return c
}

func (c *Cmd) OutputString() (string, string, error) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	c.Cmd.Stdout = stdout
	c.Cmd.Stderr = stderr
	err := c.Cmd.Run()
	return stdout.String(), stderr.String(), err
}

// CommandContext returns a wrapped exec.Cmd which kills the process group when the context is canceled.
// By default, it uses graceful termination (SIGTERM) on Unix-like systems to make the subprocesses have chances
// to clean up (e.g. remove temporary files or lock files).
func CommandContext(ctx context.Context, name string, arg ...string) *Cmd {
	c := &Cmd{Cmd: exec.CommandContext(ctx, name, arg...)} //nolint:forbidigo // wrap it
	setSysProcAttribute(c.Cmd)
	c.Cmd.Cancel = c.onCancel

	// Unlike exec.CommandContext, we use graceful termination by default to avoid corrupting data or leaving lock files behind.
	// If some processes don't respond to SIGTERM, can switch to WithOnCancelForceKill (SIGKILL) to force kill them.
	c.termGraceful = true
	return c
}
