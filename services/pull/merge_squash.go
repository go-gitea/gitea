// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// doMergeStyleSquash squashes the tracking branch on the current HEAD (=base)
func doMergeStyleSquash(ctx *mergeContext, message string) error {
	cmd := git.NewCommand(ctx, "merge", "--squash").AddDynamicArguments(trackingBranch)
	if err := runMergeCommand(ctx, repo_model.MergeStyleSquash, cmd); err != nil {
		log.Error("%-v Unable to merge --squash tracking into base: %v", ctx.pr, err)
		return err
	}

	if err := ctx.pr.Issue.LoadPoster(ctx); err != nil {
		log.Error("%-v Issue[%d].LoadPoster: %v", ctx.pr, ctx.pr.Issue.ID, err)
		return fmt.Errorf("LoadPoster: %w", err)
	}
	sig := ctx.pr.Issue.Poster.NewGitSig()
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
