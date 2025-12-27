// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package catfile

import (
	"bufio"
	"context"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"

	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"
)

// WriteCloserError wraps an io.WriteCloser with an additional CloseWithError function
type WriteCloserError interface {
	io.WriteCloser
	CloseWithError(err error) error
}

type Batch interface {
	Writer() WriteCloserError
	Reader() *bufio.Reader
	Close()
}

// batch represents an active `git cat-file --batch` or `--batch-check` invocation
// paired with the pipes that feed/read from it. Call Close to release resources.
type batch struct {
	cancel context.CancelFunc
	reader *bufio.Reader
	writer WriteCloserError
}

// NewBatch creates a new cat-file --batch process for the provided repository path.
// The returned Batch must be closed once the caller has finished with it.
func NewBatch(ctx context.Context, repoPath string) (Batch, error) {
	if err := EnsureValidGitRepository(ctx, repoPath); err != nil {
		return nil, err
	}

	var batch batch
	batch.writer, batch.reader, batch.cancel = catFileBatch(ctx, repoPath)
	return &batch, nil
}

// NewBatchCheck creates a cat-file --batch-check process for the provided repository path.
// The returned Batch must be closed once the caller has finished with it.
func NewBatchCheck(ctx context.Context, repoPath string) (Batch, error) {
	if err := EnsureValidGitRepository(ctx, repoPath); err != nil {
		return nil, err
	}

	var check batch
	check.writer, check.reader, check.cancel = catFileBatchCheck(ctx, repoPath)
	return &check, nil
}

func (b *batch) Writer() WriteCloserError {
	return b.writer
}

func (b *batch) Reader() *bufio.Reader {
	return b.reader
}

// Close stops the underlying git cat-file process and releases held resources.
func (b *batch) Close() {
	if b == nil || b.cancel == nil {
		return
	}
	b.cancel()
	b.reader = nil
	b.writer = nil
	b.cancel = nil
}

// EnsureValidGitRepository runs `git rev-parse` in the repository path to make sure
// the directory is a valid git repository. This avoids git cat-file hanging indefinitely
// when invoked in invalid paths.
func EnsureValidGitRepository(ctx context.Context, repoPath string) error {
	stder := strings.Builder{}
	err := gitcmd.NewCommand("rev-parse").
		WithDir(repoPath).
		WithStderr(&stder).
		Run(ctx)
	if err != nil {
		return gitcmd.ConcatenateError(err, stder.String())
	}
	return nil
}

// catFileBatch opens git cat-file --batch in the provided repo and returns a stdin pipe,
// a stdout reader and a cancel function.
func catFileBatch(ctx context.Context, repoPath string) (WriteCloserError, *bufio.Reader, func()) {
	batchStdinReader, batchStdinWriter := io.Pipe()
	batchStdoutReader, batchStdoutWriter := nio.Pipe(buffer.New(32 * 1024))
	ctx, ctxCancel := context.WithCancel(ctx)
	closed := make(chan struct{})
	cancel := func() {
		ctxCancel()
		_ = batchStdinWriter.Close()
		_ = batchStdoutReader.Close()
		<-closed
	}

	go func() {
		<-ctx.Done()
		cancel()
	}()

	go func() {
		stder := strings.Builder{}
		err := gitcmd.NewCommand("cat-file", "--batch").
			WithDir(repoPath).
			WithStdin(batchStdinReader).
			WithStdout(batchStdoutWriter).
			WithStderr(&stder).
			WithUseContextTimeout(true).
			Run(ctx)
		if err != nil {
			_ = batchStdoutWriter.CloseWithError(gitcmd.ConcatenateError(err, stder.String()))
			_ = batchStdinReader.CloseWithError(gitcmd.ConcatenateError(err, stder.String()))
		} else {
			_ = batchStdoutWriter.Close()
			_ = batchStdinReader.Close()
		}
		close(closed)
	}()

	batchReader := bufio.NewReaderSize(batchStdoutReader, 32*1024)
	return batchStdinWriter, batchReader, cancel
}

// catFileBatchCheck opens git cat-file --batch-check in the provided repo and returns a stdin pipe,
// a stdout reader and cancel function.
func catFileBatchCheck(ctx context.Context, repoPath string) (WriteCloserError, *bufio.Reader, func()) {
	batchStdinReader, batchStdinWriter := io.Pipe()
	batchStdoutReader, batchStdoutWriter := io.Pipe()
	ctx, ctxCancel := context.WithCancel(ctx)
	closed := make(chan struct{})
	cancel := func() {
		ctxCancel()
		_ = batchStdoutReader.Close()
		_ = batchStdinWriter.Close()
		<-closed
	}

	go func() {
		<-ctx.Done()
		cancel()
	}()

	go func() {
		stder := strings.Builder{}
		err := gitcmd.NewCommand("cat-file", "--batch-check").
			WithDir(repoPath).
			WithStdin(batchStdinReader).
			WithStdout(batchStdoutWriter).
			WithStderr(&stder).
			WithUseContextTimeout(true).
			Run(ctx)
		if err != nil {
			_ = batchStdoutWriter.CloseWithError(gitcmd.ConcatenateError(err, stder.String()))
			_ = batchStdinReader.CloseWithError(gitcmd.ConcatenateError(err, stder.String()))
		} else {
			_ = batchStdoutWriter.Close()
			_ = batchStdinReader.Close()
		}
		close(closed)
	}()

	batchReader := bufio.NewReader(batchStdoutReader)
	return batchStdinWriter, batchReader, cancel
}
