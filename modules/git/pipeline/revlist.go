// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// RevListAllObjects runs rev-list --objects --all and writes to a pipewriter
func RevListAllObjects(ctx context.Context, revListWriter *io.PipeWriter, wg *sync.WaitGroup, basePath string, errChan chan<- error) {
	defer wg.Done()
	defer revListWriter.Close()

	cmd := gitcmd.NewCommand("rev-list", "--objects", "--all")
	if err := cmd.WithDir(basePath).
		WithStdout(revListWriter).
		RunWithStderr(ctx); err != nil {
		_ = revListWriter.CloseWithError(fmt.Errorf("git rev-list --objects --all [%s]: %w", basePath, err))
		errChan <- err
	}
}

// RevListObjects run rev-list --objects from headSHA to baseSHA
func RevListObjects(ctx context.Context, revListWriter *io.PipeWriter, wg *sync.WaitGroup, tmpBasePath, headSHA, baseSHA string, errChan chan<- error) {
	defer wg.Done()
	defer revListWriter.Close()

	cmd := gitcmd.NewCommand("rev-list", "--objects").AddDynamicArguments(headSHA)
	if baseSHA != "" {
		cmd = cmd.AddArguments("--not").AddDynamicArguments(baseSHA)
	}
	if err := cmd.WithDir(tmpBasePath).
		WithStdout(revListWriter).
		RunWithStderr(ctx); err != nil {
		errChan <- fmt.Errorf("git rev-list [%s]: %w", tmpBasePath, err)
	}
}

// BlobsFromRevListObjects reads a RevListAllObjects and only selects blobs
func BlobsFromRevListObjects(revListReader *io.PipeReader, shasToCheckWriter *io.PipeWriter, wg *sync.WaitGroup) {
	defer wg.Done()
	defer revListReader.Close()
	scanner := bufio.NewScanner(revListReader)
	defer func() {
		_ = shasToCheckWriter.CloseWithError(scanner.Err())
	}()
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		fields := strings.Split(line, " ")
		if len(fields) < 2 || len(fields[1]) == 0 {
			continue
		}
		toWrite := []byte(fields[0] + "\n")
		for len(toWrite) > 0 {
			n, err := shasToCheckWriter.Write(toWrite)
			if err != nil {
				_ = revListReader.CloseWithError(err)
				break
			}
			toWrite = toWrite[n:]
		}
	}
}
