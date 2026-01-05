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
