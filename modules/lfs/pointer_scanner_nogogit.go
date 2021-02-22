// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package lfs

import (
	"bufio"
	"io"
	"strconv"
	"sync"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/pipeline"
)

// SearchPointerBlobs scans the whole repository for LFS pointer files
func SearchPointerBlobs(repo *git.Repository) ([]PointerBlob, error) {
	basePath := repo.Path

	pointerChan := make(chan PointerBlob)

	catFileCheckReader, catFileCheckWriter := io.Pipe()
	shasToBatchReader, shasToBatchWriter := io.Pipe()
	catFileBatchReader, catFileBatchWriter := io.Pipe()
	errChan := make(chan error, 1)
	wg := sync.WaitGroup{}
	wg.Add(5)

	// Create the go-routines in reverse order.

	// 5. Copy the results from the channel into the result array
	pointers := make([]PointerBlob, 0, 50)

	go func() {
		defer wg.Done()
		for pointer := range pointerChan {
			pointers = append(pointers, pointer)
		}
	}()

	// 4. Take the output of cat-file --batch and check if each file in turn
	// to see if they're pointers to files in the LFS store
	go createPointerResultsFromCatFileBatch(catFileBatchReader, &wg, pointerChan)

	// 3. Take the shas of the blobs and batch read them
	go pipeline.CatFileBatch(shasToBatchReader, catFileBatchWriter, &wg, basePath)

	// 2. From the provided objects restrict to blobs <=1k
	go pipeline.BlobsLessThan1024FromCatFileBatchCheck(catFileCheckReader, shasToBatchWriter, &wg)

	// 1. Run batch-check on all objects in the repository
	if git.CheckGitVersionAtLeast("2.6.0") != nil {
		revListReader, revListWriter := io.Pipe()
		shasToCheckReader, shasToCheckWriter := io.Pipe()
		wg.Add(2)
		go pipeline.CatFileBatchCheck(shasToCheckReader, catFileCheckWriter, &wg, basePath)
		go pipeline.BlobsFromRevListObjects(revListReader, shasToCheckWriter, &wg)
		go pipeline.RevListAllObjects(revListWriter, &wg, basePath, errChan)
	} else {
		go pipeline.CatFileBatchCheckAllObjects(catFileCheckWriter, &wg, basePath, errChan)
	}
	wg.Wait()

	select {
	case err, has := <-errChan:
		if has {
			return nil, err
		}
	default:
	}

	return pointers, nil
}

func createPointerResultsFromCatFileBatch(catFileBatchReader *io.PipeReader, wg *sync.WaitGroup, pointerChan chan<- PointerBlob) {
	defer wg.Done()
	defer catFileBatchReader.Close()

	bufferedReader := bufio.NewReader(catFileBatchReader)
	buf := make([]byte, 1025)
	for {
		// File descriptor line: sha
		sha, err := bufferedReader.ReadString(' ')
		if err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
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
		pointer, err := ReadPointerFromBuffer(pointerBuf)
		if err != nil {
			continue
		}

		pointerChan <- PointerBlob{Hash: sha, Pointer: pointer}
	}
	close(pointerChan)
}
