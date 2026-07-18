// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"bufio"
	"context"
	"io"

	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
	asymkey_service "gitea.dev/services/asymkey"
)

// This file contains commit verification functions for refs passed across in hooks

func verifyCommits(ctx context.Context, oldCommitID, newCommitID string, repo *git.Repository, env []string) error {
	var command *gitcmd.Command
	objectFormat, _ := repo.GetObjectFormat(ctx)
	if oldCommitID == objectFormat.EmptyObjectID().String() {
		// When creating a new branch, the oldCommitID is empty, by using "newCommitID --not --all":
		// List commits that are reachable by following the newCommitID, exclude "all" existing heads/tags commits
		// So, it only lists the new commits received, doesn't list the commits already present in the receiving repository
		command = gitcmd.NewCommand("rev-list").AddDynamicArguments(newCommitID).AddArguments("--not", "--all")
	} else {
		command = gitcmd.NewCommand("rev-list").AddDynamicArguments(oldCommitID + "..." + newCommitID)
	}
	// This is safe as force pushes are already forbidden
	stdoutReader, stdoutReaderClose := command.MakeStdoutPipe()
	defer stdoutReaderClose()

	err := command.WithEnv(env).
		WithDir(repo.Path).
		WithPipelineFunc(func(gitCtx gitcmd.Context) error {
			err := readAndVerifyCommitsFromShaReader(ctx, stdoutReader, repo, env)
			return gitCtx.CancelPipeline(err)
		}).
		Run(ctx)
	if err != nil && !isErrUnverifiedCommit(err) {
		log.Error("Unable to check commits from %s to %s in %s: %v", oldCommitID, newCommitID, repo.Path, err)
	}
	return err
}

func readAndVerifyCommitsFromShaReader(ctx context.Context, input io.ReadCloser, repo *git.Repository, env []string) error {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		err := readAndVerifyCommit(ctx, line, repo, env)
		if err != nil {
			return err
		}
	}
	return scanner.Err()
}

func readAndVerifyCommit(ctx context.Context, sha string, repo *git.Repository, env []string) error {
	commitID := git.MustIDFromString(sha)
	cmd := gitcmd.NewCommand("cat-file", "commit").AddDynamicArguments(sha)
	stdoutReader, stdoutReaderClose := cmd.MakeStdoutPipe()
	defer stdoutReaderClose()

	return cmd.WithEnv(env).
		WithDir(repo.Path).
		WithPipelineFunc(func(gitCtx gitcmd.Context) error {
			commit, err := git.CommitFromReader(commitID, stdoutReader)
			if err != nil {
				return err
			}
			verification := asymkey_service.ParseCommitWithSignature(ctx, commit)
			if !verification.Verified {
				return gitCtx.CancelPipeline(&errUnverifiedCommit{commit.ID.String()})
			}
			return nil
		}).
		Run(ctx)
}

type errUnverifiedCommit struct {
	sha string
}

func (e *errUnverifiedCommit) Error() string {
	return "Unverified commit: " + e.sha
}

func isErrUnverifiedCommit(err error) bool {
	_, ok := err.(*errUnverifiedCommit)
	return ok
}
