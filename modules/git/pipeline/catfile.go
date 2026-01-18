// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// CatFileBatchCheck runs cat-file with --batch-check
func CatFileBatchCheck(ctx context.Context, shasToCheckReader *io.PipeReader, catFileCheckWriter *io.PipeWriter, wg *sync.WaitGroup, tmpBasePath string) {
	defer wg.Done()
	defer shasToCheckReader.Close()
	defer catFileCheckWriter.Close()

	cmd := gitcmd.NewCommand("cat-file", "--batch-check")
	if err := cmd.WithDir(tmpBasePath).
		WithStdin(shasToCheckReader).
		WithStdout(catFileCheckWriter).
		RunWithStderr(ctx); err != nil {
		_ = catFileCheckWriter.CloseWithError(fmt.Errorf("git cat-file --batch-check [%s]: %w", tmpBasePath, err))
	}
}

// CatFileBatchCheckAllObjects runs cat-file with --batch-check --batch-all
func CatFileBatchCheckAllObjects(ctx context.Context, catFileCheckWriter *io.PipeWriter, wg *sync.WaitGroup, tmpBasePath string, errChan chan<- error) {
	defer wg.Done()
	defer catFileCheckWriter.Close()

	cmd := gitcmd.NewCommand("cat-file", "--batch-check", "--batch-all-objects")
	if err := cmd.WithDir(tmpBasePath).
		WithStdout(catFileCheckWriter).
		RunWithStderr(ctx); err != nil {
		_ = catFileCheckWriter.CloseWithError(fmt.Errorf("git cat-file --batch-check --batch-all-object [%s]: %w", tmpBasePath, err))
		errChan <- err
	}
}

// CatFileBatch runs cat-file --batch
func CatFileBatch(ctx context.Context, shasToBatchReader *io.PipeReader, catFileBatchWriter *io.PipeWriter, wg *sync.WaitGroup, tmpBasePath string) {
	defer wg.Done()
	defer shasToBatchReader.Close()
	defer catFileBatchWriter.Close()

	if err := gitcmd.NewCommand("cat-file", "--batch").
		WithDir(tmpBasePath).
		WithStdin(shasToBatchReader).
		WithStdout(catFileBatchWriter).
		RunWithStderr(ctx); err != nil {
		_ = shasToBatchReader.CloseWithError(fmt.Errorf("git rev-list [%s]: %w", tmpBasePath, err))
	}
}

// BlobsLessThan1024FromCatFileBatchCheck reads a pipeline from cat-file --batch-check and returns the blobs <1024 in size
func BlobsLessThan1024FromCatFileBatchCheck(catFileCheckReader *io.PipeReader, shasToBatchWriter *io.PipeWriter, wg *sync.WaitGroup) {
	defer wg.Done()
	defer catFileCheckReader.Close()
	scanner := bufio.NewScanner(catFileCheckReader)
	defer func() {
		_ = shasToBatchWriter.CloseWithError(scanner.Err())
	}()
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		fields := strings.Split(line, " ")
		if len(fields) < 3 || fields[1] != "blob" {
			continue
		}
		size, _ := strconv.Atoi(fields[2])
		if size > 1024 {
			continue
		}
		toWrite := []byte(fields[0] + "\n")
		for len(toWrite) > 0 {
			n, err := shasToBatchWriter.Write(toWrite)
			if err != nil {
				_ = catFileCheckReader.CloseWithError(err)
				break
			}
			toWrite = toWrite[n:]
		}
	}
}
