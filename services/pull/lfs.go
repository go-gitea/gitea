// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"bufio"
	"context"
	"io"
	"strconv"
	"sync"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/git/pipeline"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
)

// LFSPush pushes lfs objects referred to in new commits in the head repository from the base repository
func LFSPush(ctx context.Context, tmpBasePath, mergeHeadSHA, mergeBaseSHA string, pr *issues_model.PullRequest) error {
	// Now we have to implement git lfs push
	// git rev-list --objects --filter=blob:limit=1k HEAD --not base
	// pass blob shas in to git cat-file --batch-check (possibly unnecessary)
	// ensure only blobs and <=1k size then pass in to git cat-file --batch
	// to read each sha and check each as a pointer
	// Then if they are lfs -> add them to the baseRepo
	revListReader, revListWriter := io.Pipe()
	shasToCheckReader, shasToCheckWriter := io.Pipe()
	catFileCheckReader, catFileCheckWriter := io.Pipe()
	shasToBatchReader, shasToBatchWriter := io.Pipe()
	catFileBatchReader, catFileBatchWriter := io.Pipe()
	errChan := make(chan error, 1)
	wg := sync.WaitGroup{}
	wg.Add(6)
	// Create the go-routines in reverse order.

	// 6. Take the output of cat-file --batch and check if each file in turn
	// to see if they're pointers to files in the LFS store associated with
	// the head repo and add them to the base repo if so
	go createLFSMetaObjectsFromCatFileBatch(db.DefaultContext, catFileBatchReader, &wg, pr)

	// 5. Take the shas of the blobs and batch read them
	go pipeline.CatFileBatch(ctx, shasToBatchReader, catFileBatchWriter, &wg, tmpBasePath)

	// 4. From the provided objects restrict to blobs <=1k
	go pipeline.BlobsLessThan1024FromCatFileBatchCheck(catFileCheckReader, shasToBatchWriter, &wg)

	// 3. Run batch-check on the objects retrieved from rev-list
	go pipeline.CatFileBatchCheck(ctx, shasToCheckReader, catFileCheckWriter, &wg, tmpBasePath)

	// 2. Check each object retrieved rejecting those without names as they will be commits or trees
	go pipeline.BlobsFromRevListObjects(revListReader, shasToCheckWriter, &wg)

	// 1. Run rev-list objects from mergeHead to mergeBase
	go pipeline.RevListObjects(ctx, revListWriter, &wg, tmpBasePath, mergeHeadSHA, mergeBaseSHA, errChan)

	wg.Wait()
	select {
	case err, has := <-errChan:
		if has {
			return err
		}
	default:
	}
	return nil
}

func createLFSMetaObjectsFromCatFileBatch(ctx context.Context, catFileBatchReader *io.PipeReader, wg *sync.WaitGroup, pr *issues_model.PullRequest) {
	defer wg.Done()
	defer catFileBatchReader.Close()

	contentStore := lfs.NewContentStore()

	bufferedReader := bufio.NewReader(catFileBatchReader)
	buf := make([]byte, 1025)
	for {
		// File descriptor line: sha
		_, err := bufferedReader.ReadString(' ')
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
		pointer, _ := lfs.ReadPointerFromBuffer(pointerBuf)
		if !pointer.IsValid() {
			continue
		}

		exist, _ := contentStore.Exists(pointer)
		if !exist {
			continue
		}

		// Then we need to check that this pointer is in the db
		if _, err := git_model.GetLFSMetaObjectByOid(ctx, pr.HeadRepoID, pointer.Oid); err != nil {
			if err == git_model.ErrLFSObjectNotExist {
				log.Warn("During merge of: %d in %-v, there is a pointer to LFS Oid: %s which although present in the LFS store is not associated with the head repo %-v", pr.Index, pr.BaseRepo, pointer.Oid, pr.HeadRepo)
				continue
			}
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		// OK we have a pointer that is associated with the head repo
		// and is actually a file in the LFS
		// Therefore it should be associated with the base repo
		if _, err := git_model.NewLFSMetaObject(ctx, pr.BaseRepoID, pointer); err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
	}
}
