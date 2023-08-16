// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"fmt"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// getRebaseAmendMessage composes the message to amend commits in rebase merge of a pull request.
func getRebaseAmendMessage(ctx *mergeContext, baseGitRepo *git.Repository) (message string, err error) {
	// Get existing commit message.
	commitMessage, _, err := git.NewCommand(ctx, "show", "--format=%B", "-s").RunStdString(&git.RunOpts{Dir: ctx.tmpBasePath})
	if err != nil {
		return "", err
	}

	commitTitle, commitBody, _ := strings.Cut(commitMessage, "\n")
	extraVars := map[string]string{"CommitTitle": strings.TrimSpace(commitTitle), "CommitBody": strings.TrimSpace(commitBody)}

	message, body, err := getMergeMessage(ctx, baseGitRepo, ctx.pr, repo_model.MergeStyleRebase, extraVars)
	if err != nil || message == "" {
		return "", err
	}

	if len(body) > 0 {
		message = message + "\n\n" + body
	}
	return message, err
}

// Perform rebase merge without merge commit.
func doMergeRebaseFastForward(ctx *mergeContext) error {
	baseHeadSHA, err := git.GetFullCommitID(ctx, ctx.tmpBasePath, "HEAD")
	if err != nil {
		return fmt.Errorf("Failed to get full commit id for HEAD: %w", err)
	}

	cmd := git.NewCommand(ctx, "merge", "--ff-only").AddDynamicArguments(stagingBranch)
	if err := runMergeCommand(ctx, repo_model.MergeStyleRebase, cmd); err != nil {
		log.Error("Unable to merge staging into base: %v", err)
		return err
	}

	// Check if anything actually changed before we amend the message, fast forward can skip commits.
	newMergeHeadSHA, err := git.GetFullCommitID(ctx, ctx.tmpBasePath, "HEAD")
	if err != nil {
		return fmt.Errorf("Failed to get full commit id for HEAD: %w", err)
	}
	if baseHeadSHA == newMergeHeadSHA {
		return nil
	}

	// Original repo to read template from.
	baseGitRepo, err := git.OpenRepository(ctx, ctx.pr.BaseRepo.RepoPath())
	if err != nil {
		log.Error("Unable to get Git repo for rebase: %v", err)
		return err
	}
	defer baseGitRepo.Close()

	// Amend last commit message based on template, if one exists
	newMessage, err := getRebaseAmendMessage(ctx, baseGitRepo)
	if err != nil {
		log.Error("Unable to get commit message for amend: %v", err)
		return err
	}

	if newMessage != "" {
		if err := git.NewCommand(ctx, "commit", "--amend").AddOptionFormat("--message=%s", newMessage).Run(&git.RunOpts{Dir: ctx.tmpBasePath}); err != nil {
			log.Error("Unable to amend commit message: %v", err)
			return err
		}
	}

	return nil
}

// Perform rebase merge with merge commit.
func doMergeRebaseMergeCommit(ctx *mergeContext, message string) error {
	cmd := git.NewCommand(ctx, "merge").AddArguments("--no-ff", "--no-commit").AddDynamicArguments(stagingBranch)

	if err := runMergeCommand(ctx, repo_model.MergeStyleRebaseMerge, cmd); err != nil {
		log.Error("Unable to merge staging into base: %v", err)
		return err
	}
	if err := commitAndSignNoAuthor(ctx, message); err != nil {
		log.Error("Unable to make final commit: %v", err)
		return err
	}

	return nil
}

// doMergeStyleRebase rebases the tracking branch on the base branch as the current HEAD with or with a merge commit to the original pr branch
func doMergeStyleRebase(ctx *mergeContext, mergeStyle repo_model.MergeStyle, message string) error {
	if err := rebaseTrackingOnToBase(ctx, mergeStyle); err != nil {
		return err
	}

	// Checkout base branch again
	if err := git.NewCommand(ctx, "checkout").AddDynamicArguments(baseBranch).
		Run(ctx.RunOpts()); err != nil {
		log.Error("git checkout base prior to merge post staging rebase %-v: %v\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
		return fmt.Errorf("git checkout base prior to merge post staging rebase  %v: %w\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
	}
	ctx.outbuf.Reset()
	ctx.errbuf.Reset()

	if mergeStyle == repo_model.MergeStyleRebase {
		return doMergeRebaseFastForward(ctx)
	}

	return doMergeRebaseMergeCommit(ctx, message)
}
