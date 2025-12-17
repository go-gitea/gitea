// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"context"
)

type Batch struct {
	cancel context.CancelFunc
	Reader *bufio.Reader
	Writer WriteCloserError
}

// NewBatch creates a new batch for the given repository, the Close must be invoked before release the batch
func NewBatch(ctx context.Context, repoPath string) (*Batch, error) {
	// Now because of some insanity with git cat-file not immediately failing if not run in a valid git directory we need to run git rev-parse first!
	if err := ensureValidGitRepository(ctx, repoPath); err != nil {
		return nil, err
	}

	var batch Batch
	batch.Writer, batch.Reader, batch.cancel = catFileBatch(ctx, repoPath)
	return &batch, nil
}

func NewBatchCheck(ctx context.Context, repoPath string) (*Batch, error) {
	// Now because of some insanity with git cat-file not immediately failing if not run in a valid git directory we need to run git rev-parse first!
	if err := ensureValidGitRepository(ctx, repoPath); err != nil {
		return nil, err
	}

	var check Batch
	check.Writer, check.Reader, check.cancel = catFileBatchCheck(ctx, repoPath)
	return &check, nil
}

func (b *Batch) Close() {
	if b.cancel != nil {
		b.cancel()
		b.Reader = nil
		b.Writer = nil
		b.cancel = nil
	}
}
