// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"time"
)

// Fsck verifies the connectivity and validity of the objects in the database
func Fsck(ctx context.Context, repoPath string, timeout time.Duration, args TrustedCmdArgs) error {
	return NewCommand("fsck").AddArguments(args...).Run(ctx, &RunOpts{Timeout: timeout, Dir: repoPath})
}
