// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pipeline

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// RevListAllObjects runs rev-list --objects --all and writes to a pipewriter
func RevListAllObjects(revListWriter *io.PipeWriter, wg *sync.WaitGroup, basePath string, errChan chan<- error) {
	defer wg.Done()
	defer revListWriter.Close()

	stderr := new(bytes.Buffer)
	var errbuf strings.Builder
	cmd := git.NewCommand("rev-list", "--objects", "--all")
	if err := cmd.RunInDirPipeline(basePath, revListWriter, stderr); err != nil {
		log.Error("git rev-list --objects --all [%s]: %v - %s", basePath, err, errbuf.String())
		err = fmt.Errorf("git rev-list --objects --all [%s]: %v - %s", basePath, err, errbuf.String())
		_ = revListWriter.CloseWithError(err)
		errChan <- err
	}
}

// RevListObjects run rev-list --objects from headSHA to baseSHA
func RevListObjects(revListWriter *io.PipeWriter, wg *sync.WaitGroup, tmpBasePath, headSHA, baseSHA string, errChan chan<- error) {
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
