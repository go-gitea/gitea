// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strconv"

	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/git/pipeline"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"golang.org/x/sync/errgroup"
)

// LFSPush pushes lfs objects referred to in new commits in the head repository from the base repository
func LFSPush(ctx context.Context, tmpBasePath, mergeHeadSHA, mergeBaseSHA string, pr *issues_model.PullRequest) error {
	// Now we have to implement git lfs push
	// git rev-list --objects --filter=blob:limit=1k HEAD --not base
	// pass blob shas in to git cat-file --batch-check (possibly unnecessary)
	// ensure only blobs and <=1k size then pass in to git cat-file --batch
	// to read each sha and check each as a pointer
	// Then if they are lfs -> add them to the baseRepo

	cmd1RevList, cmd3BathCheck, cmd5BatchContent := gitcmd.NewCommand(), gitcmd.NewCommand(), gitcmd.NewCommand()
	cmd1RevListOut, cmd1RevListClose := cmd1RevList.MakeStdoutPipe()
	defer cmd1RevListClose()

	cmd3BatchCheckIn, cmd3BatchCheckOut, cmd3BatchCheckClose := cmd3BathCheck.MakeStdinStdoutPipe()
	defer cmd3BatchCheckClose()

	cmd5BatchContentIn, cmd5BatchContentOut, cmd5BatchContentClose := cmd5BatchContent.MakeStdinStdoutPipe()
	defer cmd5BatchContentClose()

	// Create the go-routines in reverse order (update: the order is not needed any more, the pipes are properly prepared)
	wg := &errgroup.Group{}

	// 6. Take the output of cat-file --batch and check if each file in turn
	// to see if they're pointers to files in the LFS store associated with
	// the head repo and add them to the base repo if so
	wg.Go(func() error {
		return createLFSMetaObjectsFromCatFileBatch(ctx, cmd5BatchContentOut, pr)
	})

	// 5. Take the shas of the blobs and batch read them
	wg.Go(func() error {
		return pipeline.CatFileBatch(ctx, cmd5BatchContent, tmpBasePath)
	})

	// 4. From the provided objects restrict to blobs <=1k
	wg.Go(func() error {
		return pipeline.BlobsLessThan1024FromCatFileBatchCheck(cmd3BatchCheckOut, cmd5BatchContentIn)
	})

	// 3. Run batch-check on the objects retrieved from rev-list
	wg.Go(func() error {
		return pipeline.CatFileBatchCheck(ctx, cmd3BathCheck, tmpBasePath)
	})

	// 2. Check each object retrieved rejecting those without names as they will be commits or trees
	wg.Go(func() error {
		return pipeline.BlobsFromRevListObjects(cmd1RevListOut, cmd3BatchCheckIn)
	})

	// 1. Run rev-list objects from mergeHead to mergeBase
	wg.Go(func() error {
		return pipeline.RevListObjects(ctx, cmd1RevList, tmpBasePath, mergeHeadSHA, mergeBaseSHA)
	})

	return wg.Wait()
}

func createLFSMetaObjectsFromCatFileBatch(ctx context.Context, catFileBatchReader io.ReadCloser, pr *issues_model.PullRequest) error {
	defer catFileBatchReader.Close()

	contentStore := lfs.NewContentStore()
	bufferedReader := bufio.NewReader(catFileBatchReader)
	buf := make([]byte, 1025)
	for {
		// File descriptor line: sha
		_, err := bufferedReader.ReadString(' ')
		if err != nil {
			return util.Iif(errors.Is(err, io.EOF), nil, err)
		}
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
			return err
		}
		// OK we have a pointer that is associated with the head repo
		// and is actually a file in the LFS
		// Therefore it should be associated with the base repo
		if _, err := git_model.NewLFSMetaObject(ctx, pr.BaseRepoID, pointer); err != nil {
			return err
		}
	}
}
