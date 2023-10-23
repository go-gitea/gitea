// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// This file contains commit verification functions for refs passed across in hooks

func verifyCommits(oldCommitID, newCommitID string, repo *git.Repository, env []string) error {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		log.Error("Unable to create os.Pipe for %s", repo.Path)
		return err
	}
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	var command *git.Command
	if oldCommitID == git.EmptySHA {
		// When creating a new branch, the oldCommitID is empty, by using "newCommitID --not --all":
		// List commits that are reachable by following the newCommitID, exclude "all" existing heads/tags commits
		// So, it only lists the new commits received, doesn't list the commits already present in the receiving repository
		command = git.NewCommand(repo.Ctx, "rev-list").AddDynamicArguments(newCommitID).AddArguments("--not", "--all")
	} else {
		command = git.NewCommand(repo.Ctx, "rev-list").AddDynamicArguments(oldCommitID + "..." + newCommitID)
	}
	// This is safe as force pushes are already forbidden
	err = command.Run(&git.RunOpts{
		Env:    env,
		Dir:    repo.Path,
		Stdout: stdoutWriter,
		PipelineFunc: func(ctx context.Context, cancel context.CancelFunc) error {
			_ = stdoutWriter.Close()
			err := readAndVerifyCommitsFromShaReader(stdoutReader, repo, env)
			if err != nil {
				log.Error("%v", err)
				cancel()
			}
			_ = stdoutReader.Close()
			return err
		},
	})
	if err != nil && !isErrUnverifiedCommit(err) {
		log.Error("Unable to check commits from %s to %s in %s: %v", oldCommitID, newCommitID, repo.Path, err)
	}
	return err
}

func readAndVerifyCommitsFromShaReader(input io.ReadCloser, repo *git.Repository, env []string) error {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		err := readAndVerifyCommit(line, repo, env)
		if err != nil {
			log.Error("%v", err)
			return err
		}
	}
	return scanner.Err()
}

func readAndVerifyCommit(sha string, repo *git.Repository, env []string) error {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		log.Error("Unable to create pipe for %s: %v", repo.Path, err)
		return err
	}
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()
	hash := git.MustIDFromString(sha)

	return git.NewCommand(repo.Ctx, "cat-file", "commit").AddDynamicArguments(sha).
		Run(&git.RunOpts{
			Env:    env,
			Dir:    repo.Path,
			Stdout: stdoutWriter,
			PipelineFunc: func(ctx context.Context, cancel context.CancelFunc) error {
				_ = stdoutWriter.Close()
				commit, err := git.CommitFromReader(repo, hash, stdoutReader)
				if err != nil {
					return err
				}
				verification := asymkey_model.ParseCommitWithSignature(ctx, commit)
				if !verification.Verified {
					cancel()
					return &errUnverifiedCommit{
						commit.ID.String(),
					}
				}
				return nil
			},
		})
}

type errUnverifiedCommit struct {
	sha string
}

func (e *errUnverifiedCommit) Error() string {
	return fmt.Sprintf("Unverified commit: %s", e.sha)
}

func isErrUnverifiedCommit(err error) bool {
	_, ok := err.(*errUnverifiedCommit)
	return ok
}
