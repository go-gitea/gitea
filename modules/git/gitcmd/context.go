// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"context"
)

type Context interface {
	context.Context

	// CancelWithCause is a helper function to cancel the context with a specific error cause
	// And it returns the same error for convenience, to break the PipelineFunc easily
	CancelWithCause(err error) error

	// In the future, this interface will be extended to support stdio pipe readers/writers
}

type cmdContext struct {
	context.Context
	cmd *Command
}

func (c *cmdContext) CancelWithCause(err error) error {
	c.cmd.cmdCancel(err)
	return err
}
