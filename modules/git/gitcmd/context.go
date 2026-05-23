// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"context"
)

type Context interface {
	context.Context

	// CancelPipeline is a helper function to cancel the command context (kill the command) with a specific error cause,
	// it returns the same error for convenience to break the PipelineFunc easily
	CancelPipeline(err error) error

	// In the future, this interface will be extended to support stdio pipe readers/writers
}

type cmdContext struct {
	context.Context
	cmd *Command
}

func (c *cmdContext) CancelPipeline(err error) error {
	// pipelineError is used to distinguish between:
	// * context canceled by pipeline caller with/without error (normal cancellation)
	// * context canceled by parent context (still context.Canceled error)
	// * other causes
	c.cmd.cmdCancel(pipelineError{err})
	return err
}
