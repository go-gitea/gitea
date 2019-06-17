// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
)

// LFSPush pushes lfs objects referred to in new commits in the head repository from the base repository
func LFSPush(tmpBasePath, mergeHeadSHA, mergeBaseSHA string, pr *models.PullRequest) error {
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
	go readCatFileBatch(catFileBatchReader, &wg, pr)

	// 5. Take the shas of the blobs and batch read them
	go doCatFileBatch(shasToBatchReader, catFileBatchWriter, &wg, tmpBasePath)

	// 4. From the provided objects restrict to blobs <=1k
	go readCatFileBatchCheck(catFileCheckReader, shasToBatchWriter, &wg)

	// 3. Run batch-check on the objects retrieved from rev-list
	go doCatFileBatchCheck(shasToCheckReader, catFileCheckWriter, &wg, tmpBasePath)

	// 2. Check each object retrieved rejecting those without names as they will be commits or trees
	go readRevListObjects(revListReader, shasToCheckWriter, &wg)

	// 1. Run rev-list objects from mergeHead to mergeBase
	go doRevListObjects(revListWriter, &wg, tmpBasePath, mergeHeadSHA, mergeBaseSHA, errChan)

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

func doRevListObjects(revListWriter *io.PipeWriter, wg *sync.WaitGroup, tmpBasePath, headSHA, baseSHA string, errChan chan<- error) {
	defer wg.Done()
	defer revListWriter.Close()
	stderr := new(bytes.Buffer)
	var errbuf strings.Builder
	cmd := git.NewCommand("rev-list", "--objects", headSHA, "--not", baseSHA)
	if err := cmd.RunInDirPipeline(tmpBasePath, revListWriter, stderr); err != nil {
		log.Error("git rev-list [%s]: %v - %s", tmpBasePath, err, errbuf.String())
		errChan <- fmt.Errorf("git rev-list [%s]: %v - %s", tmpBasePath, err, errbuf.String())
	}
}

func readRevListObjects(revListReader *io.PipeReader, shasToCheckWriter *io.PipeWriter, wg *sync.WaitGroup) {
	defer wg.Done()
	defer revListReader.Close()
	defer shasToCheckWriter.Close()
	scanner := bufio.NewScanner(revListReader)
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
	_ = shasToCheckWriter.CloseWithError(scanner.Err())
}

func doCatFileBatchCheck(shasToCheckReader *io.PipeReader, catFileCheckWriter *io.PipeWriter, wg *sync.WaitGroup, tmpBasePath string) {
	defer wg.Done()
	defer shasToCheckReader.Close()
	defer catFileCheckWriter.Close()

	stderr := new(bytes.Buffer)
	var errbuf strings.Builder
	cmd := git.NewCommand("cat-file", "--batch-check")
	if err := cmd.RunInDirFullPipeline(tmpBasePath, catFileCheckWriter, stderr, shasToCheckReader); err != nil {
		_ = catFileCheckWriter.CloseWithError(fmt.Errorf("git cat-file --batch-check [%s]: %v - %s", tmpBasePath, err, errbuf.String()))
	}
}

func readCatFileBatchCheck(catFileCheckReader *io.PipeReader, shasToBatchWriter *io.PipeWriter, wg *sync.WaitGroup) {
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
		size, _ := strconv.Atoi(string(fields[2]))
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

func doCatFileBatch(shasToBatchReader *io.PipeReader, catFileBatchWriter *io.PipeWriter, wg *sync.WaitGroup, tmpBasePath string) {
	defer wg.Done()
	defer shasToBatchReader.Close()
	defer catFileBatchWriter.Close()

	stderr := new(bytes.Buffer)
	var errbuf strings.Builder
	if err := git.NewCommand("cat-file", "--batch").RunInDirFullPipeline(tmpBasePath, catFileBatchWriter, stderr, shasToBatchReader); err != nil {
		_ = shasToBatchReader.CloseWithError(fmt.Errorf("git rev-list [%s]: %v - %s", tmpBasePath, err, errbuf.String()))
	}
}

func readCatFileBatch(catFileBatchReader *io.PipeReader, wg *sync.WaitGroup, pr *models.PullRequest) {
	defer wg.Done()
	defer catFileBatchReader.Close()

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
		pointer := lfs.IsPointerFile(&pointerBuf)
		if pointer == nil {
			continue
		}
		// Then we need to check that this pointer is in the db
		if _, err := pr.HeadRepo.GetLFSMetaObjectByOid(pointer.Oid); err != nil {
			if err == models.ErrLFSObjectNotExist {
				log.Warn("During merge of: %d in %-v, there is a pointer to LFS Oid: %s which although present in the LFS store is not associated with the head repo %-v", pr.Index, pr.BaseRepo, pointer.Oid, pr.HeadRepo)
				continue
			}
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		// OK we have a pointer that is associated with the head repo
		// and is actually a file in the LFS
		// Therefore it should be associated with the base repo
		pointer.RepositoryID = pr.BaseRepoID
		if _, err := models.NewLFSMetaObject(pointer); err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
	}
}
