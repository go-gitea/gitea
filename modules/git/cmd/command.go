// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"io"
	"time"
)

// RunOpts represents parameters to run the command
type RunOpts struct {
	Env            []string
	Timeout        time.Duration
	Dir            string
	Stdout, Stderr io.Writer
	Stdin          io.Reader
	PipelineFunc   func(context.Context, context.CancelFunc) error
}

// Command represents a git command
type Command interface {
	String() string
	SetParentContext(ctx context.Context) Command
	SetDescription(desc string) Command
	AddArguments(args ...string) Command
	Run(*RunOpts) error
}

// Service represents a service to create git command
type Service interface {
	NewCommand(ctx context.Context, gloablArgsLength int, args ...string) Command
}
