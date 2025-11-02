// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"io"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// catFileBatchLegacy implements the CatFileBatch interface using the "cat-file --batch" command and "cat-file --batch-check" command
// for git version < 2.36
// to align with "--batch-command", it creates the two commands for querying object contents and object info separately
// ref: https://git-scm.com/docs/git-cat-file#Documentation/git-cat-file.txt---batch
type catFileBatchLegacy struct {
	ctx          context.Context
	repoPath     string
	batchContent *catFileBatchCommunicator
	batchCheck   *catFileBatchCommunicator
}

var _ CatFileBatchCloser = (*catFileBatchLegacy)(nil)

// newCatFileBatchLegacy creates a new batch and a new batch check for the given repository, the Close must be invoked before release the batch
func newCatFileBatchLegacy(ctx context.Context, repoPath string) (*catFileBatchLegacy, error) {
	if err := ensureValidGitRepository(ctx, repoPath); err != nil {
		return nil, err
	}

	return &catFileBatchLegacy{
		ctx:      ctx,
		repoPath: repoPath,
	}, nil
}

func (b *catFileBatchLegacy) getBatchContent() *catFileBatchCommunicator {
	if b.batchContent != nil {
		return b.batchContent
	}
	b.batchContent = newCatFileBatch(b.ctx, b.repoPath, gitcmd.NewCommand("cat-file", "--batch"))
	return b.batchContent
}

func (b *catFileBatchLegacy) getBatchCheck() *catFileBatchCommunicator {
	if b.batchCheck != nil {
		return b.batchCheck
	}
	b.batchCheck = newCatFileBatch(b.ctx, b.repoPath, gitcmd.NewCommand("cat-file", "--batch-check"))
	return b.batchCheck
}

func (b *catFileBatchLegacy) QueryContent(obj string) (BufferedReader, error) {
	_, err := io.WriteString(b.getBatchContent().writer, obj+"\n")
	return b.getBatchContent().reader, err
}

func (b *catFileBatchLegacy) QueryInfo(obj string) (BufferedReader, error) {
	_, err := io.WriteString(b.getBatchCheck().writer, obj+"\n")
	return b.getBatchCheck().reader, err
}

func (b *catFileBatchLegacy) Close() {
	if b.batchContent != nil {
		b.batchContent.Close()
		b.batchContent = nil
	}
	if b.batchCheck != nil {
		b.batchCheck.Close()
		b.batchCheck = nil
	}
}
