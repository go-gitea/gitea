// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build race

package git

import (
	"context"
	"testing"
	"time"
)

func TestRunWithContextNoTimeout(t *testing.T) {
	maxLoops := 10

	// 'git --version' does not block so it must be finished before the timeout triggered.
	cmd := NewCommand("--version")
	for i := 0; i < maxLoops; i++ {
		if err := cmd.Run(t.Context(), &RunOpts{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunWithContextTimeout(t *testing.T) {
	maxLoops := 10

	// 'git hash-object --stdin' blocks on stdin so we can have the timeout triggered.
	cmd := NewCommand("hash-object", "--stdin")
	for i := 0; i < maxLoops; i++ {
		if err := cmd.Run(t.Context(), &RunOpts{Timeout: 1 * time.Millisecond}); err != nil {
			if err != context.DeadlineExceeded {
				t.Fatalf("Testing %d/%d: %v", i, maxLoops, err)
			}
		}
	}
}
