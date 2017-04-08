// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build race

package git

import (
	"testing"
	"time"
)

func TestRunInDirTimeoutPipelineNoTimeout(t *testing.T) {

	maxLoops := 1000

	// 'git --version' does not block so it must be finished before the timeout triggered.
	cmd := NewCommand("--version")
	for i := 0; i < maxLoops; i++ {
		if err := cmd.RunInDirTimeoutPipeline(-1, "", nil, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunInDirTimeoutPipelineAlwaysTimeout(t *testing.T) {

	maxLoops := 1000

	// 'git hash-object --stdin' blocks on stdin so we can have the timeout triggered.
	cmd := NewCommand("hash-object --stdin")
	for i := 0; i < maxLoops; i++ {
		if err := cmd.RunInDirTimeoutPipeline(1*time.Microsecond, "", nil, nil); err != nil {
			// 'context deadline exceeded' when the error is returned by exec.Start
			// 'signal: killed' when the error is returned by exec.Wait
			//  It depends on the point of the time (before or after exec.Start returns) at which the timeout is triggered.
			if err.Error() != "context deadline exceeded" && err.Error() != "signal: killed" {
				t.Fatalf("Testing %d/%d: %v", i, maxLoops, err)
			}
		}
	}
}
