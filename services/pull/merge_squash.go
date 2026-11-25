// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

func getAuthorSignatureSquash(ctx context.Context, pr *issues_model.PullRequest, gitRepo *git.Repository, baseRef, headRef git.RefName) (*git.Signature, error) {
	if err := pr.Issue.LoadPoster(ctx); err != nil {
		log.Error("%-v Issue[%d].LoadPoster: %v", pr, pr.Issue.ID, err)
		return nil, err
	}

	// Try to get a signature from the same user in one of the commits, as the
	// poster email might be private or commits might have a different signature
	// than the primary email address of the poster.
	commits, err := gitRepo.CommitsBetween(ctx, headRef, baseRef, -1)
	if err != nil {
		log.Error("%-v Unable to get commits between: head and tracking branch: %v", pr, err)
		return nil, err
	}

	uniqueEmails := make(container.Set[string])
	for _, commit := range commits {
		if commit.Author != nil && uniqueEmails.Add(commit.Author.Email) {
			commitUser, _ := user_model.GetUserByEmail(ctx, commit.Author.Email)
			if commitUser != nil && commitUser.ID == pr.Issue.Poster.ID {
				return commit.Author, nil
			}
		}
	}

	return pr.Issue.Poster.NewGitSig(), nil
}

func squashCommitMessage(message string, author, committer *git.Signature) string {
	if setting.Repository.PullRequest.AddCoCommitterTrailers && committer.String() != author.String() {
		return git.CommitMessageAddTrailer(message, git.CoAuthoredByTrailer, author.String())
	}
	return message
}

// doMergeStyleSquash squashes the tracking branch on the current HEAD (=base)
func doMergeStyleSquash(ctx *mergeContext, message string) error {
	gitRepo, err := git.OpenRepositoryLocal(ctx, ctx.tmpBasePath)
	if err != nil {
		return err
	}
	defer gitRepo.Close()
	sig, err := getAuthorSignatureSquash(ctx, ctx.pr, gitRepo, git.RefNameHead, git.RefNameFromBranch(tmpRepoTrackingBranch))
	if err != nil {
		return fmt.Errorf("getAuthorSignatureSquash: %w", err)
	}

	cmdMerge := gitcmd.NewCommand("merge", "--squash").AddDynamicArguments(tmpRepoTrackingBranch)
	if err := runMergeCommand(ctx, repo_model.MergeStyleSquash, cmdMerge); err != nil {
		log.Error("%-v Unable to merge --squash tracking into base: %v", ctx.pr, err)
		return err
	}

	message = squashCommitMessage(message, sig, ctx.committer)
	cmdCommit := gitcmd.NewCommand("commit", "--allow-empty").
		AddOptionFormat("--author='%s <%s>'", sig.Name, sig.Email)
	if err = git.AddObjectMessageArgument(cmdCommit, git.ObjectCommit, message); err != nil {
		return err
	}
	addCommitSigningOptions(cmdCommit, ctx.signKey)
	if err := ctx.PrepareGitCmd(cmdCommit).RunWithStderr(ctx); err != nil {
		log.Error("git commit %-v: %v\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), err.Stderr())
		return fmt.Errorf("git commit [%s:%s -> %s:%s]: %w\n%s\n%s", ctx.pr.HeadRepo.FullName(), ctx.pr.HeadBranch, ctx.pr.BaseRepo.FullName(), ctx.pr.BaseBranch, err, ctx.outbuf.String(), err.Stderr())
	}
	ctx.outbuf.Reset()
	return nil
}
