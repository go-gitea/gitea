// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package lfs

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/pipeline"
	"gitea.dev/modules/util"

	"golang.org/x/sync/errgroup"
)

// SearchPointerBlobs scans the whole repository for LFS pointer files
func SearchPointerBlobs(ctx context.Context, repo *git.Repository, pointerChan chan<- PointerBlob) error {
	cmd1AllObjs, cmd3BatchContent := gitcmd.NewCommand(), gitcmd.NewCommand()

	cmd1AllObjsStdout, cmd1AllObjsStdoutClose := cmd1AllObjs.MakeStdoutPipe()
	defer cmd1AllObjsStdoutClose()

	cmd3BatchContentIn, cmd3BatchContentOut, cmd3BatchContentClose := cmd3BatchContent.MakeStdinStdoutPipe()
	defer cmd3BatchContentClose()

	// Create the go-routines in reverse order (update: the order is not needed any more, the pipes are properly prepared)
	wg := errgroup.Group{}
	// 4. Take the output of cat-file --batch and check if each file in turn
	// to see if they're pointers to files in the LFS store
	wg.Go(func() error {
		return createPointerResultsFromCatFileBatch(cmd3BatchContentOut, pointerChan)
	})

	// 3. Take the shas of the blobs and batch read them
	wg.Go(func() error {
		return pipeline.CatFileBatch(ctx, cmd3BatchContent, repo)
	})

	// 2. From the provided objects restrict to blobs <=1k
	wg.Go(func() error {
		return pipeline.BlobsLessThan1024FromCatFileBatchCheck(cmd1AllObjsStdout, cmd3BatchContentIn)
	})

	// 1. Run batch-check on all objects in the repository
	wg.Go(func() error {
		return pipeline.CatFileBatchCheckAllObjects(ctx, cmd1AllObjs, repo)
	})
	err := wg.Wait()
	close(pointerChan)
	return err
}

// SearchPointerBlobsInRange scans only the objects reachable from headRefs but
// not from excludeRefs for LFS pointer files. This is used for incremental
// mirror sync to avoid scanning the entire repository.
func SearchPointerBlobsInRange(ctx context.Context, repo *git.Repository, headRefs, excludeRefs []string, pointerChan chan<- PointerBlob, errChan chan<- error) {
	// Feed refs via stdin to avoid ARG_MAX limits on large repositories.
	var stdinBuf bytes.Buffer
	for _, ref := range headRefs {
		stdinBuf.WriteString(ref)
		stdinBuf.WriteByte('\n')
	}
	for _, ref := range excludeRefs {
		stdinBuf.WriteByte('^')
		stdinBuf.WriteString(ref)
		stdinBuf.WriteByte('\n')
	}

	cmdRevList := gitcmd.NewCommand()
	cmdBatchCheck := gitcmd.NewCommand()
	cmdBatchContent := gitcmd.NewCommand()

	revListStdout, revListClose := cmdRevList.MakeStdoutPipe()
	defer revListClose()

	batchCheckIn, batchCheckOut, batchCheckClose := cmdBatchCheck.MakeStdinStdoutPipe()
	defer batchCheckClose()

	batchContentIn, batchContentOut, batchContentClose := cmdBatchContent.MakeStdinStdoutPipe()
	defer batchContentClose()

	wg := errgroup.Group{}
	wg.Go(func() error {
		return createPointerResultsFromCatFileBatch(batchContentOut, pointerChan)
	})
	wg.Go(func() error {
		return pipeline.CatFileBatch(ctx, cmdBatchContent, repo)
	})
	wg.Go(func() error {
		return pipeline.BlobsLessThan1024FromCatFileBatchCheck(batchCheckOut, batchContentIn)
	})
	wg.Go(func() error {
		return pipeline.CatFileBatchCheck(ctx, cmdBatchCheck, repo)
	})
	wg.Go(func() error {
		return pipeline.BlobsFromRevListObjects(revListStdout, batchCheckIn)
	})
	wg.Go(func() error {
		return cmdRevList.AddArguments("rev-list", "--objects", "--ignore-missing", "--stdin").
			WithStdinBytes(stdinBuf.Bytes()).
			WithRepo(repo).
			RunWithStderr(ctx)
	})

	err := wg.Wait()
	close(pointerChan)
	if err != nil {
		errChan <- err
	}
	close(errChan)
}

func createPointerResultsFromCatFileBatch(catFileBatchReader io.ReadCloser, pointerChan chan<- PointerBlob) error {
	defer catFileBatchReader.Close()

	bufferedReader := bufio.NewReader(catFileBatchReader)
	buf := make([]byte, 1025)

	for {
		// File descriptor line: sha
		sha, err := bufferedReader.ReadString(' ')
		if err != nil {
			return util.Iif(errors.Is(err, io.EOF), nil, err)
		}
		sha = strings.TrimSpace(sha)
		// Throw away the blob
		if _, err := bufferedReader.ReadString(' '); err != nil {
			return err
		}
		sizeStr, err := bufferedReader.ReadString('\n')
		if err != nil {
			return err
		}
		size, err := strconv.Atoi(sizeStr[:len(sizeStr)-1])
		if err != nil {
			return err
		}
		pointerBuf := buf[:size+1]
		if _, err := io.ReadFull(bufferedReader, pointerBuf); err != nil {
			return err
		}
		pointerBuf = pointerBuf[:size]
		// Now we need to check if the pointerBuf is an LFS pointer
		pointer, _ := ReadPointerFromBuffer(pointerBuf)
		if !pointer.IsValid() {
			continue
		}
		pointerChan <- PointerBlob{Hash: sha, Pointer: pointer}
	}
}
