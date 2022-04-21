// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build !gogit
// +build !gogit

package lfs

import (
	"bufio"
	"context"
	"io"
	"strconv"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/pipeline"
)

// SearchPointerBlobs scans the whole repository for LFS pointer files
func SearchPointerBlobs(ctx context.Context, repo *git.Repository, pointerChan chan<- PointerBlob, errChan chan<- error) {
	basePath := repo.Path

	closers := []git.CloserError{}
	closeAll := func(err error) {
		for _, closer := range closers {
			_ = closer.CloseWithError(err)
		}
	}
	defer closeAll(nil)

	fail := func(err error) {
		errChan <- err
		close(pointerChan)
		close(errChan)
	}

	catFileCheckReader, catFileCheckWriter, err := git.Pipe()
	if err != nil {
		fail(err)
		return
	}
	closers = append(closers, catFileCheckReader, catFileCheckWriter)

	shasToBatchReader, shasToBatchWriter, err := git.Pipe()
	if err != nil {
		fail(err)
		return
	}
	closers = append(closers, shasToBatchReader, shasToBatchWriter)

	catFileBatchReader, catFileBatchWriter, err := git.Pipe()
	if err != nil {
		fail(err)
		return
	}
	closers = append(closers, catFileBatchReader, catFileBatchWriter)

	wg := sync.WaitGroup{}
	wg.Add(4)

	// Create the go-routines in reverse order.

	// 4. Take the output of cat-file --batch and check if each file in turn
	// to see if they're pointers to files in the LFS store
	go createPointerResultsFromCatFileBatch(ctx, catFileBatchReader, &wg, pointerChan)

	// 3. Take the shas of the blobs and batch read them
	go pipeline.CatFileBatch(ctx, shasToBatchReader, catFileBatchWriter, &wg, basePath)

	// 2. From the provided objects restrict to blobs <=1k
	go pipeline.BlobsLessThan1024FromCatFileBatchCheck(catFileCheckReader, shasToBatchWriter, &wg)

	// 1. Run batch-check on all objects in the repository
	if git.CheckGitVersionAtLeast("2.6.0") != nil {
		revListReader, revListWriter, err := git.Pipe()
		if err != nil {
			wg.Done()
			closeAll(err)
			wg.Wait()
			fail(err)
			return
		}
		closers = append(closers, revListReader, revListWriter)
		shasToCheckReader, shasToCheckWriter, err := git.Pipe()
		if err != nil {
			wg.Done()
			closeAll(err)
			wg.Wait()
			fail(err)
			return
		}
		closers = append(closers, shasToCheckReader, shasToCheckWriter)

		wg.Add(2)
		go pipeline.CatFileBatchCheck(ctx, shasToCheckReader, catFileCheckWriter, &wg, basePath)
		go pipeline.BlobsFromRevListObjects(revListReader, shasToCheckWriter, &wg)
		go pipeline.RevListAllObjects(ctx, revListWriter, &wg, basePath, errChan)
	} else {
		go pipeline.CatFileBatchCheckAllObjects(ctx, catFileCheckWriter, &wg, basePath, errChan)
	}
	wg.Wait()

	close(pointerChan)
	close(errChan)
}

// createPointerResultsFromCatFileBatch does not call git
func createPointerResultsFromCatFileBatch(ctx context.Context, catFileBatchReader git.ReadCloserError, wg *sync.WaitGroup, pointerChan chan<- PointerBlob) {
	defer wg.Done()
	defer catFileBatchReader.Close()

	bufferedReader := bufio.NewReader(catFileBatchReader)
	buf := make([]byte, 1025)

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		default:
		}

		// File descriptor line: sha
		sha, err := bufferedReader.ReadString(' ')
		if err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		sha = strings.TrimSpace(sha)
		// Throw away the blob
		if _, err := bufferedReader.ReadString(' '); err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		sizeStr, err := bufferedReader.ReadString('\n')
		if err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		size, err := strconv.Atoi(sizeStr[:len(sizeStr)-1])
		if err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		pointerBuf := buf[:size+1]
		if _, err := io.ReadFull(bufferedReader, pointerBuf); err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
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
