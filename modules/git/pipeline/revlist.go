// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"bufio"
	"context"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// RevListObjects run rev-list --objects from headSHA to baseSHA
func RevListObjects(ctx context.Context, cmd *gitcmd.Command, tmpBasePath, headSHA, baseSHA string) error {
	cmd.AddArguments("rev-list", "--objects").AddDynamicArguments(headSHA)
	if baseSHA != "" {
		cmd = cmd.AddArguments("--not").AddDynamicArguments(baseSHA)
	}
	return cmd.WithDir(tmpBasePath).RunWithStderr(ctx)
}

// BlobsFromRevListObjects reads a RevListAllObjects and only selects blobs
func BlobsFromRevListObjects(in io.ReadCloser, out io.WriteCloser) error {
	defer out.Close()
	scanner := bufio.NewScanner(in)
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
			n, err := out.Write(toWrite)
			if err != nil {
				return err
			}
			toWrite = toWrite[n:]
		}
	}
	return scanner.Err()
}
