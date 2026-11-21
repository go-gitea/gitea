// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"io"
	"strings"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

// catFileBatchCommand implements the CatFileBatch interface using the "cat-file --batch-command" command
// for git version >= 2.36
// ref: https://git-scm.com/docs/git-cat-file#Documentation/git-cat-file.txt---batch-command
type catFileBatchCommand struct {
	ctx   context.Context
	repo  RepositoryFacade
	batch *catFileBatchCommunicator
}

var _ CatFileBatch = (*catFileBatchCommand)(nil)

func newCatFileBatchCommand(ctx context.Context, repo RepositoryFacade) *catFileBatchCommand {
	return &catFileBatchCommand{ctx: ctx, repo: repo}
}

func (b *catFileBatchCommand) getBatch(callerInfo string) *catFileBatchCommunicator {
	if b.batch != nil {
		return b.batch
	}
	b.batch = newCatFileBatch(b.ctx, b.repo, gitcmd.NewCommand("cat-file", "--batch-command").WithParentCallerInfo(callerInfo))
	return b.batch
}

func (b *catFileBatchCommand) Context() context.Context {
	return b.ctx
}

func (b *catFileBatchCommand) QueryContent(obj string) (*CatFileObject, io.Reader, error) {
	if strings.Contains(obj, "\n") {
		setting.PanicInDevOrTesting("invalid object name with newline: %q", obj)
	}
	return b.getBatch(util.CallerFuncName(1)).queryContent("contents " + obj + "\n")
}

func (b *catFileBatchCommand) QueryInfo(obj string) (*CatFileObject, error) {
	if strings.Contains(obj, "\n") {
		setting.PanicInDevOrTesting("invalid object name with newline: %q", obj)
	}
	return b.getBatch(util.CallerFuncName(1)).query("info " + obj + "\n")
}

func (b *catFileBatchCommand) Close() {
	if b.batch != nil {
		b.batch.Close()
		b.batch = nil
	}
}
