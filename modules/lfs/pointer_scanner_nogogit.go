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

// SearchPointerFiles scans the whole repository for LFS pointer files
func SearchPointerFiles(repo *git.Repository) ([]PointerBlob, error) {
	basePath := repo.Path

	pointerChan := make(chan PointerBlob)

	catFileCheckReader, catFileCheckWriter := io.Pipe()
	shasToBatchReader, shasToBatchWriter := io.Pipe()
	catFileBatchReader, catFileBatchWriter := io.Pipe()
	errChan := make(chan error, 1)
	wg := sync.WaitGroup{}
	wg.Add(5)

	pointers := make([]PointerBlob, 0, 50)

	go func() {
		defer wg.Done()
		for pointer := range pointerChan {
			pointers = append(pointers, pointer)
		}
	}()
	go createPointerResultsFromCatFileBatch(catFileBatchReader, &wg, pointerChan)
	go pipeline.CatFileBatch(shasToBatchReader, catFileBatchWriter, &wg, basePath)
	go pipeline.BlobsLessThan1024FromCatFileBatchCheck(catFileCheckReader, shasToBatchWriter, &wg)
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
		pointer := TryReadPointerFromBuffer(pointerBuf)
		if pointer == nil {
			continue
		}

		pointerChan <- PointerBlob{Hash: sha, Pointer: pointer}
	}
	close(pointerChan)
}