// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"context"
)

type batchCatFile struct {
	cancel context.CancelFunc
	Reader *bufio.Reader
	Writer WriteCloserError
}

func (b *batchCatFile) Close() {
	if b.cancel != nil {
		b.cancel()
		b.Reader = nil
		b.Writer = nil
		b.cancel = nil
	}
}

type Batch interface {
	Write([]byte) (int, error)
	WriteCheck([]byte) (int, error)
	Reader() *bufio.Reader
	CheckReader() *bufio.Reader
	Close()
}

// batchCatFileWithCheck implements the Batch interface using the "cat-file --batch" command and "cat-file --batch-check" command
// ref: https://git-scm.com/docs/git-cat-file#Documentation/git-cat-file.txt---batch
// To align with --batch-command, we creates the two commands both at the same time if git version is lower than 2.36
type batchCatFileWithCheck struct {
	batch      *batchCatFile
	batchCheck *batchCatFile
}

var _ Batch = &batchCatFileWithCheck{}

// newBatchCatFileWithCheck creates a new batch and a new batch check for the given repository, the Close must be invoked before release the batch
func newBatchCatFileWithCheck(ctx context.Context, repoPath string) (*batchCatFileWithCheck, error) {
	// Now because of some insanity with git cat-file not immediately failing if not run in a valid git directory we need to run git rev-parse first!
	if err := ensureValidGitRepository(ctx, repoPath); err != nil {
		return nil, err
	}

	var batch batchCatFile
	batch.Writer, batch.Reader, batch.cancel = catFileBatch(ctx, repoPath, "--batch")

	var check batchCatFile
	check.Writer, check.Reader, check.cancel = catFileBatchCheck(ctx, repoPath)

	return &batchCatFileWithCheck{
		batch:      &batch,
		batchCheck: &check,
	}, nil
}

func (b *batchCatFileWithCheck) Write(bs []byte) (int, error) {
	return b.batch.Writer.Write(bs)
}

func (b *batchCatFileWithCheck) WriteCheck(bs []byte) (int, error) {
	return b.batchCheck.Writer.Write(bs)
}

func (b *batchCatFileWithCheck) Reader() *bufio.Reader {
	return b.batch.Reader
}

func (b *batchCatFileWithCheck) CheckReader() *bufio.Reader {
	return b.batchCheck.Reader
}

func (b *batchCatFileWithCheck) Close() {
	if b.batch != nil {
		b.batch.Close()
		b.batch = nil
	}
	if b.batchCheck != nil {
		b.batchCheck.Close()
		b.batchCheck = nil
	}
}

// batchCommandCatFile implements the Batch interface using the "cat-file --batch-command" command
// ref: https://git-scm.com/docs/git-cat-file#Documentation/git-cat-file.txt---batch-command
type batchCommandCatFile struct {
	batch *batchCatFile
}

var _ Batch = &batchCommandCatFile{}

func newBatchCommandCatFile(ctx context.Context, repoPath string) (*batchCommandCatFile, error) {
	// Now because of some insanity with git cat-file not immediately failing if not run in a valid git directory we need to run git rev-parse first!
	if err := ensureValidGitRepository(ctx, repoPath); err != nil {
		return nil, err
	}

	var batch batchCatFile
	batch.Writer, batch.Reader, batch.cancel = catFileBatch(ctx, repoPath, "--batch-command")

	return &batchCommandCatFile{
		batch: &batch,
	}, nil
}

func (b *batchCommandCatFile) Write(bs []byte) (int, error) {
	return b.batch.Writer.Write(append([]byte("contents "), bs...))
}

func (b *batchCommandCatFile) WriteCheck(bs []byte) (int, error) {
	return b.batch.Writer.Write(append([]byte("info "), bs...))
}

func (b *batchCommandCatFile) Reader() *bufio.Reader {
	return b.batch.Reader
}

func (b *batchCommandCatFile) CheckReader() *bufio.Reader {
	return b.batch.Reader
}

func (b *batchCommandCatFile) Close() {
	if b.batch != nil {
		b.batch.Close()
		b.batch = nil
	}
}

func NewBatch(ctx context.Context, repoPath string) (Batch, error) {
	if DefaultFeatures().SupportCatFileBatchCommand {
		return newBatchCommandCatFile(ctx, repoPath)
	}
	return newBatchCatFileWithCheck(ctx, repoPath)
}
