// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"errors"
	"net/http"
	"os"

	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
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
	canCreatePullRequest   *bool
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

// CanCreatePullRequest returns true if pusher can create pull requests
func (ctx *preReceiveContext) CanCreatePullRequest() bool {
	if ctx.canCreatePullRequest == nil {
		ctx.canCreatePullRequest = new(ctx.Repo.Permission.CanRead(unit.TypePullRequests))
	}
	return *ctx.canCreatePullRequest
}

// AssertCreatePullRequest returns true if can create pull requests
func (ctx *preReceiveContext) AssertCreatePullRequest() bool {
	if !ctx.CanCreatePullRequest() {
		if ctx.Written() {
			return false
		}
		ctx.PrivateUserErrorf(http.StatusForbidden, "User permission denied for creating pull-request.")
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
			preReceiveTag(ourCtx, newCommitID, refFullName)
		case git.DefaultFeatures().SupportProcReceive && refFullName.IsFor():
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

	defaultBranch := repo.DefaultBranch
	if ctx.opts.IsWiki && repo.DefaultWikiBranch != "" {
		defaultBranch = repo.DefaultWikiBranch
	}
	if branchName == defaultBranch && newCommitID == objectFormat.EmptyObjectID().String() {
		ctx.PrivateUserErrorf(http.StatusForbidden, "Branch %s is the default branch and cannot be deleted", branchName)
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
	// 1. Detect and prevent deletion of the branch
	if newCommitID == objectFormat.EmptyObjectID().String() {
		ctx.PrivateUserErrorf(http.StatusForbidden, "Branch %s is protected from deletion", branchName)
		return
	}

	isForcePush := false

	// 2. Disallow force pushes to protected branches
	if oldCommitID != objectFormat.EmptyObjectID().String() {
		output, _, err := gitcmd.NewCommand("rev-list", "--max-count=1").
			AddDynamicArguments(oldCommitID, "^"+newCommitID).
			WithEnv(ctx.env).WithRepo(repo).RunStdString(ctx)
		if err != nil {
			ctx.PrivateInternalErrorf("Unable to detect force push between %s and %s in %s: %v", oldCommitID, newCommitID, repo.FullName(), err)
			return
		} else if len(output) > 0 {
			if protectBranch.CanForcePush {
				isForcePush = true
			} else {
				ctx.PrivateUserErrorf(http.StatusForbidden, "Branch %s is protected from force push", branchName)
				return
			}
		}
	}

	// 3. Enforce require signed commits
	if protectBranch.RequireSignedCommits {
		err := verifyCommits(ctx, oldCommitID, newCommitID, gitRepo, ctx.env)
		if err != nil {
			errUnverified, ok := err.(*errUnverifiedCommit)
			if !ok {
				ctx.PrivateInternalErrorf("Unable to check commits from %s to %s: %v", oldCommitID, newCommitID, err)
				return
			}
			ctx.PrivateUserErrorf(http.StatusForbidden, "Branch %s is protected from unverified commit %s", branchName, errUnverified.sha)
			return
		}
	}

	// Now there are several tests which can be overridden:
	//
	// 4. Check protected file patterns - this is overridable from the UI
	changedProtectedfiles := false
	protectedFilePath := ""

	globs := protectBranch.GetProtectedFilePatterns()
	if len(globs) > 0 {
		_, err := pull_service.CheckFileProtection(ctx, gitRepo, branchName, oldCommitID, newCommitID, globs, 1, ctx.env)
		if err != nil {
			errFilePathProtected, ok := errors.AsType[pull_service.ErrFilePathProtected](err)
			if !ok {
				ctx.PrivateInternalErrorf("Unable to check file protection for commits from %s to %s: %v", oldCommitID, newCommitID, err)
				return
			}

			changedProtectedfiles = true
			protectedFilePath = errFilePathProtected.Path
		}
	}

	// 5. Check if the doer is allowed to push (and force-push if the incoming push is a force-push)
	var canPush bool
	if ctx.opts.UserID == user_model.DeployKeyUserID {
		// This flag is only ever true if protectBranch.CanForcePush is true
		if isForcePush {
			canPush = !changedProtectedfiles && protectBranch.CanPush && (!protectBranch.EnableForcePushAllowlist || protectBranch.ForcePushAllowlistDeployKeys)
		} else {
			canPush = !changedProtectedfiles && protectBranch.CanPush && (!protectBranch.EnableWhitelist || protectBranch.WhitelistDeployKeys)
		}
	} else {
		if isForcePush {
			canPush = !changedProtectedfiles && protectBranch.CanUserForcePush(ctx, ctx.Doer)
		} else {
			canPush = !changedProtectedfiles && protectBranch.CanUserPush(ctx, ctx.Doer)
		}
	}

	// 6. If we're not allowed to push directly
	if !canPush {
		// Is this is a merge from the UI/API?
		if ctx.opts.PullRequestID == 0 {
			// 6a. If we're not merging from the UI/API then there are two ways we got here:
			//
			// We are changing a protected file, and we're not allowed to do that
			if changedProtectedfiles {
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
		if changedProtectedfiles {
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

func preReceiveTag(ctx *preReceiveContext, newCommitID string, refFullName git.RefName) {
	if !ctx.assertCanWriteRef(refFullName) {
		return
	}

	tagName := refFullName.TagName()

	// a claimed name can never be created or moved again, deleting it needs its release gone first
	immutable, err := repo_model.IsTagImmutable(ctx, ctx.Repo.Repository, tagName)
	if immutable && err == nil && git.IsEmptyCommitID(newCommitID) {
		immutable, err = repo_model.HasImmutableRelease(ctx, ctx.Repo.Repository.ID, tagName)
	}
	if err != nil {
		ctx.PrivateInternalErrorf("unable to check immutable tag %s: %v", tagName, err)
		return
	}
	if immutable {
		ctx.PrivateUserErrorf(http.StatusForbidden, "Tag %s is immutable", tagName)
		return
	}

	if ctx.protectedTags == nil {
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
	if !ctx.AssertCreatePullRequest() {
		return
	}

	if ctx.Repo.Repository.IsEmpty {
		ctx.PrivateUserErrorf(http.StatusForbidden, "Can't create pull request for an empty repository.")
		return
	}

	if ctx.opts.IsWiki {
		ctx.PrivateUserErrorf(http.StatusForbidden, "Pull requests are not supported on the wiki.")
		return
	}

	_, _, err := agit.GetAgitBranchInfo(ctx, ctx.Repo.Repository.ID, refFullName.ForBranchName())
	if err != nil {
		if !errors.Is(err, util.ErrNotExist) {
			ctx.PrivateUserErrorf(http.StatusForbidden, "Unexpected ref: %s", refFullName)
		} else {
			ctx.PrivateInternalErrorf("Unable to get branch info for ref %s: %v", refFullName, err)
		}
	}
}

func generateGitEnv(opts *private.HookOptions) (env []string) {
	env = os.Environ()
	if opts.GitAlternativeObjectDirectories != "" {
		env = append(env,
			private.GitAlternativeObjectDirectories+"="+opts.GitAlternativeObjectDirectories)
	}
	if opts.GitObjectDirectory != "" {
		env = append(env,
			private.GitObjectDirectory+"="+opts.GitObjectDirectory)
	}
	if opts.GitQuarantinePath != "" {
		env = append(env,
			private.GitQuarantinePath+"="+opts.GitQuarantinePath)
	}
	return env
}
