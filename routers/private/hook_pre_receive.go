// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"errors"
	"net/http"
	"os"

	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unit"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/private"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/agit"
	gitea_context "gitea.dev/services/context"
	pull_service "gitea.dev/services/pull"
)

type preReceiveContext struct {
	*gitea_context.PrivateContext
	env  []string
	opts *private.HookOptions

	// this context should only contain shared variables, mutable variables like "current branch name" shouldn't be put here
	canWriteCodeUnitCached *bool
	protectedTags          []*git_model.ProtectedTag
}

func (ctx *preReceiveContext) canWriteCodeUnit() bool {
	if ctx.canWriteCodeUnitCached == nil {
		ctx.canWriteCodeUnitCached = new(ctx.Repo.Permission.CanWrite(unit.TypeCode))
	}
	return *ctx.canWriteCodeUnitCached
}

// canWriteCodeRef returns true if pusher can write to the code ref (branch/tag/commit)
func (ctx *preReceiveContext) canWriteCodeRef(refFullName git.RefName) bool {
	if ctx.canWriteCodeUnit() {
		return true
	}
	// then check whether if the pusher is a maintainer who can write the PR author's head repo branch
	if !refFullName.IsBranch() {
		return false
	}
	return issues_model.CanMaintainerWriteToBranch(ctx, ctx.Repo.Permission, refFullName.BranchName(), ctx.Doer)
}

// assertCanWriteRef returns true if pusher can write to the code ref, otherwise it responds with 403 Forbidden and returns false
func (ctx *preReceiveContext) assertCanWriteRef(refFullName git.RefName) bool {
	if !ctx.canWriteCodeRef(refFullName) {
		if ctx.Written() {
			return false
		}
		ctx.PrivateUserErrorf(http.StatusForbidden, "User permission denied for writing.")
		return false
	}
	return true
}

func assertCanCreatePullRequest(ctx *gitea_context.PrivateContext, opts *private.HookOptions) bool {
	canReadPullRequests := ctx.Repo.Permission.CanRead(unit.TypePullRequests)
	if !canReadPullRequests || opts.IsWiki {
		ctx.PrivateUserErrorf(http.StatusForbidden, "%s", util.Iif(canReadPullRequests, "Pull requests are not supported on the wiki.", "User permission denied for creating pull-request."))
		return false
	}
	return true
}

// HookPreReceive checks whether a individual commit is acceptable
func HookPreReceive(ctx *gitea_context.PrivateContext) {
	opts := web.GetForm[*private.HookOptions](ctx)
	if !loadContextDoerPermission(ctx, opts.UserID, opts.UserExtDoerData) {
		return
	}

	ourCtx := &preReceiveContext{
		PrivateContext: ctx,
		env:            generateGitEnv(opts), // Generate git environment for checking commits
		opts:           opts,
	}

	// Iterate across the provided old commit IDs
	for i := range opts.OldCommitIDs {
		oldCommitID := opts.OldCommitIDs[i]
		newCommitID := opts.NewCommitIDs[i]
		refFullName := opts.RefFullNames[i]

		switch {
		case refFullName.IsBranch():
			preReceiveBranch(ourCtx, oldCommitID, newCommitID, refFullName)
		case refFullName.IsTag():
			preReceiveTag(ourCtx, refFullName)
		case refFullName.IsFor():
			preReceiveFor(ourCtx, refFullName)
		default:
			ourCtx.assertCanWriteRef(refFullName)
		}
		if ctx.Written() {
			return
		}
	}

	ctx.PlainText(http.StatusOK, "ok")
}

func preReceiveBranch(ctx *preReceiveContext, oldCommitID, newCommitID string, refFullName git.RefName) {
	branchName := refFullName.BranchName()

	if !ctx.assertCanWriteRef(refFullName) {
		return
	}

	repo := ctx.Repo.Repository
	gitRepo := ctx.Repo.GitRepo
	objectFormat := ctx.Repo.GetObjectFormat()
	isDeletingBranch := newCommitID == objectFormat.EmptyObjectID().String()

	defaultBranch := repo.DefaultBranch
	if ctx.opts.IsWiki && repo.DefaultWikiBranch != "" {
		defaultBranch = repo.DefaultWikiBranch
	}

	if isDeletingBranch && branchName == defaultBranch {
		ctx.PrivateUserErrorf(http.StatusForbidden, "Branch %s is the default branch and cannot be deleted", branchName)
		return
	}
	if isDeletingBranch && branchName == repo.GetPullRequestTargetBranch(ctx) {
		ctx.PrivateUserErrorf(http.StatusForbidden, "Branch %s is protected from deletion", branchName)
		return
	}

	protectBranch, err := git_model.GetFirstMatchProtectedBranchRule(ctx, repo.ID, branchName)
	if err != nil {
		ctx.PrivateInternalErrorf("Unable to get protected branch: %v", err)
		return
	}

	// Allow pushes to non-protected branches
	if protectBranch == nil {
		return
	}
	protectBranch.Repo = repo

	// This ref is a protected branch.
	//
	// First of all we need to enforce absolutely:
	//
	// 1. Enforce branch deletion permissions
	if isDeletingBranch {
		if !protectBranch.CanUserDelete(ctx, ctx.Doer, ctx.Repo.Permission) {
			ctx.PrivateUserErrorf(http.StatusForbidden, "Branch %s is protected from deletion", branchName)
		}
		return
	}

	// 2. Disallow force pushes to protected branches
	isForcePush, ok := checkForcePush(ctx, protectBranch, branchName, oldCommitID, newCommitID)
	if !ok {
		return
	}

	// 3. Enforce require signed commits
	if !checkSignedCommits(ctx, protectBranch, branchName, oldCommitID, newCommitID) {
		return
	}

	// Now there are several tests which can be overridden:
	//
	// 4. Check protected file patterns - this is overridable from the UI
	changedProtectedFiles, protectedFilePath, ok := checkProtectedFiles(ctx, protectBranch, branchName, oldCommitID, newCommitID)
	if !ok {
		return
	}

	// 5. Check if the doer is allowed to push (and force-push if the incoming push is a force-push)
	var canPush bool
	if isForcePush {
		canPush = !changedProtectedFiles && protectBranch.CanUserForcePush(ctx, ctx.Doer, ctx.Repo.Permission)
	} else {
		canPush = !changedProtectedFiles && protectBranch.CanUserPush(ctx, ctx.Doer, ctx.Repo.Permission)
	}

	// 6. If we're not allowed to push directly
	if !canPush {
		// Is this is a merge from the UI/API?
		if ctx.opts.PullRequestID == 0 {
			// 6a. If we're not merging from the UI/API then there are two ways we got here:
			//
			// We are changing a protected file, and we're not allowed to do that
			if changedProtectedFiles {
				ctx.PrivateUserErrorf(http.StatusForbidden, "Branch %s is protected from changing file %s", branchName, protectedFilePath)
				return
			}

			// Allow commits that only touch unprotected files
			globs := protectBranch.GetUnprotectedFilePatterns()
			if len(globs) > 0 {
				unprotectedFilesOnly, err := pull_service.CheckUnprotectedFiles(ctx, gitRepo, branchName, oldCommitID, newCommitID, globs, ctx.env)
				if err != nil {
					ctx.PrivateInternalErrorf("Unable to check file protection for commits from %s to %s: %v", oldCommitID, newCommitID, err)
					return
				}
				if unprotectedFilesOnly {
					// Commit only touches unprotected files, this is allowed
					return
				}
			}

			// Or we're simply not able to push to this protected branch
			if isForcePush {
				ctx.PrivateUserErrorf(http.StatusForbidden, "Not allowed to force-push to protected branch %s", branchName)
				return
			}
			ctx.PrivateUserErrorf(http.StatusForbidden, "Not allowed to push to protected branch %s", branchName)
			return
		}
		// 6b. Merge (from UI or API)

		// Get the PR, user and permissions for the user in the repository
		pr, err := issues_model.GetPullRequestByID(ctx, ctx.opts.PullRequestID)
		if err != nil {
			ctx.PrivateInternalErrorf("Unable to get PullRequest %d Error: %v", ctx.opts.PullRequestID, err)
			return
		}

		// Now check if the user is allowed to merge PRs for this repository
		// Note: we can use ctx.perm and ctx.user directly as they will have been loaded above
		allowedMerge, err := pull_service.IsUserAllowedToMerge(ctx, pr, ctx.Repo.Permission, ctx.Doer)
		if err != nil {
			ctx.PrivateInternalErrorf("Error calculating if allowed to merge: %v", err)
			return
		}

		if !allowedMerge {
			ctx.PrivateUserErrorf(http.StatusForbidden, "Not allowed to push to protected branch %s", branchName)
			return
		}

		// If we can bypass branch protection we can ignore status checks, reviews and protected files
		if git_model.CanBypassBranchProtection(ctx, protectBranch, ctx.Doer, ctx.Repo.Permission.IsAdmin()) {
			return
		}

		// Now if we're not an admin - we can't overwrite protected files so fail now
		if changedProtectedFiles {
			ctx.PrivateUserErrorf(http.StatusForbidden, "Branch %s is protected from changing file %s", branchName, protectedFilePath)
			return
		}

		// Check all status checks and reviews are ok
		if err := pull_service.CheckPullBranchProtections(ctx, pr, true); err != nil {
			if errors.Is(err, pull_service.ErrNotReadyToMerge) {
				ctx.PrivateUserErrorf(http.StatusForbidden, "Not allowed to push to protected branch %s and pr #%d is not ready to be merged: %s", branchName, ctx.opts.PullRequestID, err.Error())
				return
			}
			ctx.PrivateInternalErrorf("Unable to get status of pull request %d: %v", ctx.opts.PullRequestID, err)
			return
		}
	}
}

// checkProtectedFiles reports whether the push changes a file that matches the
// branch's protected file patterns, and the first such path. It does not reject
// the push for that, since step 6 decides; ok is false only if the check failed.
func checkProtectedFiles(ctx *preReceiveContext, protectBranch *git_model.ProtectedBranch, branchName, oldCommitID, newCommitID string) (changed bool, path string, ok bool) {
	globs := protectBranch.GetProtectedFilePatterns()
	if len(globs) == 0 {
		return false, "", true
	}

	_, err := pull_service.CheckFileProtection(ctx, ctx.Repo.GitRepo, branchName, oldCommitID, newCommitID, globs, 1, ctx.env)
	if err == nil {
		return false, "", true
	}
	errFilePathProtected, isProtected := errors.AsType[pull_service.ErrFilePathProtected](err)
	if !isProtected {
		ctx.PrivateInternalErrorf("Unable to check file protection for commits from %s to %s: %v", oldCommitID, newCommitID, err)
		return false, "", false
	}
	return true, errFilePathProtected.Path, true
}

// checkForcePush reports whether the push rewrites the branch history. It
// rejects the push when the branch does not allow force pushes; ok is false
// if the push was rejected.
func checkForcePush(ctx *preReceiveContext, protectBranch *git_model.ProtectedBranch, branchName, oldCommitID, newCommitID string) (isForcePush, ok bool) {
	// A new branch has no history to rewrite.
	if oldCommitID == ctx.Repo.GetObjectFormat().EmptyObjectID().String() {
		return false, true
	}

	repo := ctx.Repo.Repository
	output, _, err := gitcmd.NewCommand("rev-list", "--max-count=1").
		AddDynamicArguments(oldCommitID, "^"+newCommitID).
		WithEnv(ctx.env).WithRepo(repo).RunStdString(ctx)
	if err != nil {
		ctx.PrivateInternalErrorf("Unable to detect force push between %s and %s in %s: %v", oldCommitID, newCommitID, repo.FullName(), err)
		return false, false
	}
	if len(output) == 0 {
		return false, true
	}
	if !protectBranch.CanForcePush {
		ctx.PrivateUserErrorf(http.StatusForbidden, "Branch %s is protected from force push", branchName)
		return false, false
	}
	return true, true
}

// checkSignedCommits rejects the push when the branch requires signed commits
// and a new commit is not verified. It returns false if the push was rejected.
func checkSignedCommits(ctx *preReceiveContext, protectBranch *git_model.ProtectedBranch, branchName, oldCommitID, newCommitID string) bool {
	if !protectBranch.RequireSignedCommits {
		return true
	}
	err := verifyCommits(ctx, oldCommitID, newCommitID, ctx.Repo.GitRepo, ctx.env)
	if err != nil {
		errUnverified, ok := err.(*errUnverifiedCommit)
		if !ok {
			ctx.PrivateInternalErrorf("Unable to check commits from %s to %s: %v", oldCommitID, newCommitID, err)
			return false
		}
		ctx.PrivateUserErrorf(http.StatusForbidden, "Branch %s is protected from unverified commit %s", branchName, errUnverified.sha)
		return false
	}
	return true
}

func preReceiveTag(ctx *preReceiveContext, refFullName git.RefName) {
	if !ctx.assertCanWriteRef(refFullName) {
		return
	}

	tagName := refFullName.TagName()

	if ctx.protectedTags == nil {
		var err error
		ctx.protectedTags, err = git_model.GetProtectedTags(ctx, ctx.Repo.Repository.ID)
		if err != nil {
			ctx.PrivateInternalErrorf("Unable to get protected tags: %v", err)
			return
		}
		ctx.protectedTags = util.SliceNilAsEmpty(ctx.protectedTags)
	}

	isAllowed, err := git_model.IsUserAllowedToControlTag(ctx, ctx.protectedTags, tagName, ctx.opts.UserID)
	if err != nil {
		ctx.PrivateInternalErrorf("unable to check allowed tags: %v", err)
		return
	}
	if !isAllowed {
		ctx.PrivateUserErrorf(http.StatusForbidden, "Tag %s is protected", tagName)
		return
	}
}

func preReceiveFor(ctx *preReceiveContext, refFullName git.RefName) {
	if !assertCanCreatePullRequest(ctx.PrivateContext, ctx.opts) {
		return
	} else if ctx.Repo.Repository.IsEmpty {
		ctx.PrivateUserErrorf(http.StatusForbidden, "Can't create pull request for an empty repository.")
	} else if _, _, err := agit.GetAgitBranchInfo(ctx, ctx.Repo.Repository.ID, refFullName.ForBranchName()); err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.PrivateUserErrorf(http.StatusForbidden, "Unexpected ref: %s", refFullName)
		} else {
			ctx.PrivateInternalErrorf("Unable to get branch info for ref %s: %v", refFullName, err)
		}
	}
}

func generateGitEnv(opts *private.HookOptions) (env []string) {
	env = os.Environ()
	if opts.GitAlternativeObjectDirectories != "" {
		env = append(env, private.GitAlternativeObjectDirectories+"="+opts.GitAlternativeObjectDirectories)
	}
	if opts.GitObjectDirectory != "" {
		env = append(env, private.GitObjectDirectory+"="+opts.GitObjectDirectory)
	}
	if opts.GitQuarantinePath != "" {
		env = append(env, private.GitQuarantinePath+"="+opts.GitQuarantinePath)
	}
	return env
}
