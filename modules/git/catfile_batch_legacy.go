// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/util"
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

func newCatFileBatchLegacy(ctx context.Context, repoPath string) (*catFileBatchLegacy, error) {
	if _, err := os.Stat(repoPath); err != nil {
		return nil, util.NewNotExistErrorf("repo %q doesn't exist", filepath.Base(repoPath))
	}
	return &catFileBatchLegacy{ctx: ctx, repoPath: repoPath}, nil
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

func (b *catFileBatchLegacy) QueryContent(obj string) (*CatFileObject, BufferedReader, error) {
	_, err := io.WriteString(b.getBatchContent().reqWriter, obj+"\n")
	if err != nil {
		return nil, nil, err
	}
	info, err := catFileBatchParseInfoLine(b.getBatchContent().respReader)
	if err != nil {
		return nil, nil, err
	}
	return info, b.getBatchContent().respReader, nil
}

func (b *catFileBatchLegacy) QueryInfo(obj string) (*CatFileObject, error) {
	_, err := io.WriteString(b.getBatchCheck().reqWriter, obj+"\n")
	if err != nil {
		return nil, err
	}
	return catFileBatchParseInfoLine(b.getBatchCheck().respReader)
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
