// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"bufio"
	"context"
	"io"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// CatFileBatchCheck runs cat-file with --batch-check
func CatFileBatchCheck(ctx context.Context, cmd *gitcmd.Command, tmpBasePath string) error {
	cmd.AddArguments("cat-file", "--batch-check")
	return cmd.WithDir(tmpBasePath).RunWithStderr(ctx)
}

// CatFileBatchCheckAllObjects runs cat-file with --batch-check --batch-all
func CatFileBatchCheckAllObjects(ctx context.Context, cmd *gitcmd.Command, tmpBasePath string) error {
	return cmd.AddArguments("cat-file", "--batch-check", "--batch-all-objects").WithDir(tmpBasePath).RunWithStderr(ctx)
}

// CatFileBatch runs cat-file --batch
func CatFileBatch(ctx context.Context, cmd *gitcmd.Command, tmpBasePath string) error {
	return cmd.AddArguments("cat-file", "--batch").WithDir(tmpBasePath).RunWithStderr(ctx)
}

// BlobsLessThan1024FromCatFileBatchCheck reads a pipeline from cat-file --batch-check and returns the blobs <1024 in size
func BlobsLessThan1024FromCatFileBatchCheck(in io.ReadCloser, out io.WriteCloser) error {
	defer out.Close()
	scanner := bufio.NewScanner(in)
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
			n, err := out.Write(toWrite)
			if err != nil {
				return err
			}
			toWrite = toWrite[n:]
		}
	}
	return scanner.Err()
}
