// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package gitrepo

import (
	"bufio"
	"context"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// WalkReferences walks all the references from the repository
func WalkReferences(ctx context.Context, repo Repository, walkfn func(sha1, refname string) error) (int, error) {
	return WalkShowRef(ctx, repo, nil, 0, 0, walkfn)
}

// callShowRef return refs, if limit = 0 it will not limit
func callShowRef(ctx context.Context, repo Repository, trimPrefix string, extraArgs gitcmd.TrustedCmdArgs, skip, limit int) (branchNames []string, countAll int, err error) {
	countAll, err = WalkShowRef(ctx, repo, extraArgs, skip, limit, func(_, branchName string) error {
		branchName = strings.TrimPrefix(branchName, trimPrefix)
		branchNames = append(branchNames, branchName)

		return nil
	})
	return branchNames, countAll, err
}

func WalkShowRef(ctx context.Context, repo Repository, extraArgs gitcmd.TrustedCmdArgs, skip, limit int, walkfn func(sha1, refname string) error) (countAll int, err error) {
	i := 0
	args := gitcmd.TrustedCmdArgs{"for-each-ref", "--format=%(objectname) %(refname)"}
	args = append(args, extraArgs...)
	cmd := gitcmd.NewCommand(args...)
	stdoutReader, stdoutReaderClose := cmd.MakeStdoutPipe()
	defer stdoutReaderClose()
	cmd.WithPipelineFunc(func(c gitcmd.Context) error {
		bufReader := bufio.NewReader(stdoutReader)
		for i < skip {
			_, isPrefix, err := bufReader.ReadLine()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			if !isPrefix {
				i++
			}
		}
		for limit == 0 || i < skip+limit {
			// The output of show-ref is simply a list:
			// <sha> SP <ref> LF
			sha, err := bufReader.ReadString(' ')
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}

			branchName, err := bufReader.ReadString('\n')
			if err == io.EOF {
				// This shouldn't happen... but we'll tolerate it for the sake of peace
				return nil
			}
			if err != nil {
				return err
			}

			if len(branchName) > 0 {
				branchName = branchName[:len(branchName)-1]
			}

			if len(sha) > 0 {
				sha = sha[:len(sha)-1]
			}

			err = walkfn(sha, branchName)
			if err != nil {
				return err
			}
			i++
		}
		// count all refs
		for limit != 0 {
			_, isPrefix, err := bufReader.ReadLine()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			if !isPrefix {
				i++
			}
		}
		return nil
	})
	err = RunCmdWithStderr(ctx, repo, cmd)
	if errPipeline := gitcmd.ErrorAsPipeline(err); errPipeline != nil {
		return i, errPipeline // keep the old behavior: return pipeline error directly
	}
	return i, err
}
