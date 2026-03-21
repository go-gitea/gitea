// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"bufio"
	"context"
	"errors"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"

	"golang.org/x/sync/errgroup"
)

func fillResultNameRev(ctx context.Context, basePath string, results []*LFSResult) error {
	// Should really use a go-git function here but name-rev is not completed and recapitulating it is not simple
	wg := errgroup.Group{}
	cmd := gitcmd.NewCommand("name-rev", "--stdin", "--name-only", "--always").WithDir(basePath)
	stdin, stdinClose := cmd.MakeStdinPipe()
	stdout, stdoutClose := cmd.MakeStdoutPipe()
	defer stdinClose()
	defer stdoutClose()

	wg.Go(func() error {
		scanner := bufio.NewScanner(stdout)
		i := 0
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) == 0 {
				continue
			}
			result := results[i]
			result.FullCommitName = line
			result.BranchName = strings.Split(line, "~")[0]
			i++
		}
		return scanner.Err()
	})
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
	err := cmd.RunWithStderr(ctx)
	return errors.Join(err, wg.Wait())
}
