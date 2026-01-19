// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"context"
	"fmt"
	"io"
	"sync"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// NameRevStdin runs name-rev --stdin
func NameRevStdin(ctx context.Context, shasToNameReader *io.PipeReader, nameRevStdinWriter *io.PipeWriter, wg *sync.WaitGroup, tmpBasePath string) {
	defer wg.Done()
	defer shasToNameReader.Close()
	defer nameRevStdinWriter.Close()

	if err := gitcmd.NewCommand("name-rev", "--stdin", "--name-only", "--always").
		WithDir(tmpBasePath).
		WithStdin(shasToNameReader).
		WithStdout(nameRevStdinWriter).
		RunWithStderr(ctx); err != nil {
		_ = shasToNameReader.CloseWithError(fmt.Errorf("git name-rev [%s]: %w", tmpBasePath, err))
	}
}
