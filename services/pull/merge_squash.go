// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"fmt"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// doMergeStyleSquash gets a commit author signature for squash commits
func getAuthorSignatureSquash(ctx *mergeContext) (*git.Signature, error) {
	if err := ctx.pr.Issue.LoadPoster(ctx); err != nil {
		log.Error("%-v Issue[%d].LoadPoster: %v", ctx.pr, ctx.pr.Issue.ID, err)
		return nil, err
	}

	// Try to get an signature from the same user in one of the commits, as the
	// poster email might be private or commits might have a different signature
	// than the primary email address of the poster.
	gitRepo, err := git.OpenRepository(ctx, ctx.tmpBasePath)
	if err != nil {
		log.Error("%-v Unable to open base repository: %v", ctx.pr, err)
		return nil, err
	}
	defer gitRepo.Close()

	commits, err := gitRepo.CommitsBetweenIDs(trackingBranch, "HEAD")
	if err != nil {
		log.Error("%-v Unable to get commits between: %s %s: %v", ctx.pr, "HEAD", trackingBranch, err)
		return nil, err
	}

	uniqueEmails := make(container.Set[string])
	for _, commit := range commits {
		if commit.Author != nil && uniqueEmails.Add(commit.Author.Email) {
			commitUser, _ := user_model.GetUserByEmail(ctx, commit.Author.Email)
			if commitUser != nil && commitUser.ID == ctx.pr.Issue.Poster.ID {
				return commit.Author, nil
			}
		}
	}

	return ctx.pr.Issue.Poster.NewGitSig(), nil
}

// doMergeStyleSquash squashes the tracking branch on the current HEAD (=base)
func doMergeStyleSquash(ctx *mergeContext, message string) error {
	sig, err := getAuthorSignatureSquash(ctx)
	if err != nil {
		return fmt.Errorf("getAuthorSignatureSquash: %w", err)
	}

	cmdMerge := git.NewCommand("merge", "--squash").AddDynamicArguments(trackingBranch)
	if err := runMergeCommand(ctx, repo_model.MergeStyleSquash, cmdMerge); err != nil {
		log.Error("%-v Unable to merge --squash tracking into base: %v", ctx.pr, err)
		return err
	}

	if setting.Repository.PullRequest.AddCoCommitterTrailers && ctx.committer.String() != sig.String() {
		// add trailer
		if !strings.Contains(message, fmt.Sprintf("Co-authored-by: %s", sig.String())) {
			message += fmt.Sprintf("\nCo-authored-by: %s", sig.String())
		}
		message += fmt.Sprintf("\nCo-committed-by: %s\n", sig.String())
	}
	cmdCommit := git.NewCommand("commit").
		AddOptionFormat("--author='%s <%s>'", sig.Name, sig.Email).
		AddOptionFormat("--message=%s", message)
	if ctx.signKeyID == "" {
		cmdCommit.AddArguments("--no-gpg-sign")
	} else {
		cmdCommit.AddOptionFormat("-S%s", ctx.signKeyID)
	}
	if err := cmdCommit.Run(ctx, ctx.RunOpts()); err != nil {
		log.Error("git commit %-v: %v\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
		return fmt.Errorf("git commit [%s:%s -> %s:%s]: %w\n%s\n%s", ctx.pr.HeadRepo.FullName(), ctx.pr.HeadBranch, ctx.pr.BaseRepo.FullName(), ctx.pr.BaseBranch, err, ctx.outbuf.String(), ctx.errbuf.String())
	}
	ctx.outbuf.Reset()
	ctx.errbuf.Reset()
	return nil
}
