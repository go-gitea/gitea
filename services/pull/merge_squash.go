// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// doMergeStyleSquash gets a commit author signature for squash commits
func getAuthorSignatureSquash(ctx *mergeContext) (*git.Signature, error) {
	if err := ctx.pr.Issue.LoadPoster(ctx); err != nil {
		return nil, fmt.Errorf("LoadPoster: %w", err)
	}

	// Try to get an signature from the same user in one of the commits, as the
	// poster email might be private or commits might have a different signature
	// than the primary email address of the poster.
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, ctx.tmpBasePath)
	if err != nil {
		return nil, fmt.Errorf("Unable to open repository: Error %v", err)
	}
	defer closer.Close()

	commits, err := gitRepo.CommitsBetweenIDs(trackingBranch, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("Unable to get commits between: %s %s Error %v", "HEAD", trackingBranch, err)
	}

	for _, commit := range commits {
		if commit.Author != nil {
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
		log.Error("getAuthorSignatureSquash: %v", err)
		return err
	}

	cmd := git.NewCommand(ctx, "merge", "--squash").AddDynamicArguments(trackingBranch)
	if err := runMergeCommand(ctx, repo_model.MergeStyleSquash, cmd); err != nil {
		log.Error("%-v Unable to merge --squash tracking into base: %v", ctx.pr, err)
		return err
	}

	if len(ctx.signArg) == 0 {
		if err := git.NewCommand(ctx, "commit").
			AddOptionFormat("--author='%s <%s>'", sig.Name, sig.Email).
			AddOptionFormat("--message=%s", message).
			Run(ctx.RunOpts()); err != nil {
			log.Error("git commit %-v: %v\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
			return fmt.Errorf("git commit [%s:%s -> %s:%s]: %w\n%s\n%s", ctx.pr.HeadRepo.FullName(), ctx.pr.HeadBranch, ctx.pr.BaseRepo.FullName(), ctx.pr.BaseBranch, err, ctx.outbuf.String(), ctx.errbuf.String())
		}
	} else {
		if setting.Repository.PullRequest.AddCoCommitterTrailers && ctx.committer.String() != sig.String() {
			// add trailer
			message += fmt.Sprintf("\nCo-authored-by: %s\nCo-committed-by: %s\n", sig.String(), sig.String())
		}
		if err := git.NewCommand(ctx, "commit").
			AddArguments(ctx.signArg...).
			AddOptionFormat("--author='%s <%s>'", sig.Name, sig.Email).
			AddOptionFormat("--message=%s", message).
			Run(ctx.RunOpts()); err != nil {
			log.Error("git commit %-v: %v\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
			return fmt.Errorf("git commit [%s:%s -> %s:%s]: %w\n%s\n%s", ctx.pr.HeadRepo.FullName(), ctx.pr.HeadBranch, ctx.pr.BaseRepo.FullName(), ctx.pr.BaseBranch, err, ctx.outbuf.String(), ctx.errbuf.String())
		}
	}
	ctx.outbuf.Reset()
	ctx.errbuf.Reset()
	return nil
}
