// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/util"
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
	if _, err := os.Stat(repoPath); err != nil {
		return nil, util.NewNotExistErrorf("repo %q doesn't exist", filepath.Base(repoPath))
	}
	return &catFileBatchCommand{ctx: ctx, repoPath: repoPath}, nil
}

func (b *catFileBatchCommand) getBatch() *catFileBatchCommunicator {
	if b.batch != nil {
		return b.batch
	}
	b.batch = newCatFileBatch(b.ctx, b.repoPath, gitcmd.NewCommand("cat-file", "--batch-command"))
	return b.batch
}

func (b *catFileBatchCommand) QueryContent(obj string) (*CatFileObject, BufferedReader, error) {
	_, err := b.getBatch().reqWriter.Write([]byte("contents " + obj + "\n"))
	if err != nil {
		return nil, nil, err
	}
	info, err := catFileBatchParseInfoLine(b.getBatch().respReader)
	if err != nil {
		return nil, nil, err
	}
	return info, b.getBatch().respReader, nil
}

func (b *catFileBatchCommand) QueryInfo(obj string) (*CatFileObject, error) {
	_, err := b.getBatch().reqWriter.Write([]byte("info " + obj + "\n"))
	if err != nil {
		return nil, err
	}
	return catFileBatchParseInfoLine(b.getBatch().respReader)
}

func (b *catFileBatchCommand) Close() {
	if b.batch != nil {
		b.batch.Close()
		b.batch = nil
	}
}
