// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"bufio"
	"context"
	"errors"
	"strings"

	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"

	"golang.org/x/sync/errgroup"
)

func fillResultNameRev(ctx context.Context, repo git.RepositoryFacade, results []*LFSResult) error {
	wg := errgroup.Group{}
	cmd := gitcmd.NewCommand("name-rev", "--stdin", "--name-only", "--always").WithRepo(repo)
	stdin, stdinClose := cmd.MakeStdinPipe()
	defer stdinClose()

	wg.Go(func() error {
		defer stdinClose()
		for _, result := range results {
			_, err := stdin.Write([]byte(result.SHA))
			if err != nil {
				return err
			}
			_, err = stdin.Write([]byte{'\n'})
			if err != nil {
				return err
			}
		}
		return nil
	})

	idx := 0
	err := gitcmd.RunWithScannerSplit(ctx, cmd, bufio.ScanLines, func(ctx gitcmd.Context, chunk []byte) error {
		line := string(chunk)
		if idx < len(results) {
			results[idx].FullCommitName = line
			results[idx].BranchName = strings.Split(line, "~")[0]
		}
		idx++
		return nil
	})
	return errors.Join(err, wg.Wait())
}
