// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// catFileBatchCommand implements the CatFileBatch interface using the "cat-file --batch-command" command
// for git version >= 2.36
// ref: https://git-scm.com/docs/git-cat-file#Documentation/git-cat-file.txt---batch-command
type catFileBatchCommand struct {
	ctx      context.Context
	repoPath string
	batch    *catFileBatchCommunicator
}

var _ CatFileBatch = (*catFileBatchCommand)(nil)

func newCatFileBatchCommand(ctx context.Context, repoPath string) (*catFileBatchCommand, error) {
	if err := ensureValidGitRepository(ctx, repoPath); err != nil {
		return nil, err
	}
	return &catFileBatchCommand{
		ctx:      ctx,
		repoPath: repoPath,
	}, nil
}

func (b *catFileBatchCommand) getBatch() *catFileBatchCommunicator {
	if b.batch != nil {
		return b.batch
	}
	b.batch = newCatFileBatch(b.ctx, b.repoPath, gitcmd.NewCommand("cat-file", "--batch-command"))
	return b.batch
}

// QueryContent sends a "contents <obj>" command to the cat-file --batch-command process
// it actually can receive a reference name, revspec, or object ID
func (b *catFileBatchCommand) QueryContent(obj string) (*CatFileObject, BufferedReader, error) {
	_, err := b.getBatch().writer.Write([]byte("contents " + obj + "\n"))
	if err != nil {
		return nil, nil, err
	}
	info, err := catFileBatchParseInfoLine(b.getBatch().reader)
	if err != nil {
		return nil, nil, err
	}
	return info, b.getBatch().reader, nil
}

// QueryContent sends a "info <obj>" command to the cat-file --batch-command process
// it actually can receive a reference name, revspec, or object ID
func (b *catFileBatchCommand) QueryInfo(obj string) (*CatFileObject, error) {
	_, err := b.getBatch().writer.Write([]byte("info " + obj + "\n"))
	if err != nil {
		return nil, err
	}
	return catFileBatchParseInfoLine(b.getBatch().reader)
}

func (b *catFileBatchCommand) Close() {
	if b.batch != nil {
		b.batch.Close()
		b.batch = nil
	}
}
