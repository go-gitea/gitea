// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/cache"
	"gitea.dev/modules/cachegroup"
	"gitea.dev/modules/git"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/private"
	repo_module "gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	gitea_context "gitea.dev/services/context"
	pull_service "gitea.dev/services/pull"
	repo_service "gitea.dev/services/repository"
)

func hookPostReceiveCollectPushUpdates(opts *private.HookOptions, repo *repo_model.Repository) []*repo_module.PushUpdateOptions {
	updates := make([]*repo_module.PushUpdateOptions, 0, len(opts.OldCommitIDs))
	for i := range opts.OldCommitIDs {
		refFullName := opts.RefFullNames[i]

		// Only trigger activity updates for changes to branches or
		// tags.  Updates to other refs (eg, refs/notes, refs/changes,
		// or other less-standard refs spaces are ignored since there
		// may be a very large number of them).
		if refFullName.IsBranch() || refFullName.IsTag() {
			option := &repo_module.PushUpdateOptions{
				RefFullName:  refFullName,
				OldCommitID:  opts.OldCommitIDs[i],
				NewCommitID:  opts.NewCommitIDs[i],
				PusherID:     opts.UserID,
				PusherName:   opts.UserName,
				RepoUserName: repo.OwnerName,
				RepoName:     repo.Name,
			}
			updates = append(updates, option)
		}
	}
	return updates
}

func hookPostReceiveSyncDatabaseBranches(ctx *gitea_context.PrivateContext, opts *private.HookOptions, repo *repo_model.Repository, updates []*repo_module.PushUpdateOptions) bool {
	branchesToSync := make([]*repo_module.PushUpdateOptions, 0, len(updates))
	for _, update := range updates {
		if !update.RefFullName.IsBranch() {
			continue
		}
		if update.IsDelRef() {
			if err := git_model.MarkBranchAsDeleted(ctx, repo.ID, update.RefFullName.BranchName(), update.PusherID); err != nil {
				ctx.PrivateError(http.StatusInternalServerError, err, fmt.Sprintf("failed to mark branch %s as deleted", update.RefFullName))
				return false
			}
		} else {
			branchesToSync = append(branchesToSync, update)
			// TODO: should we return the error and return the error when pushing? Currently it will log the error and not prevent the pushing
			pull_service.UpdatePullsRefs(ctx, repo, update)
		}
	}

	if len(branchesToSync) == 0 {
		return true
	}

	gitRepo, err := gitrepo.RepositoryFromRequestContextOrOpen(ctx, repo)
	if err != nil {
		ctx.PrivateError(http.StatusInternalServerError, err, "failed to open repository")
		return false
	}

	branchNames := make([]string, 0, len(branchesToSync))
	commitIDs := make([]string, 0, len(branchesToSync))
	for _, update := range branchesToSync {
		branchNames = append(branchNames, update.RefFullName.BranchName())
		commitIDs = append(commitIDs, update.NewCommitID)
	}

	if err = repo_service.SyncBranchesToDB(ctx, repo.ID, opts.UserID, branchNames, commitIDs, gitRepo.GetCommit); err != nil {
		ctx.PrivateError(http.StatusInternalServerError, err, "failed to sync branch to DB")
		return false
	}
	return true
}

// HookPostReceive updates services and users
func HookPostReceive(ctx *gitea_context.PrivateContext) {
	opts := web.GetForm(ctx).(*private.HookOptions)
	if opts.IsWiki {
		setting.PanicInDevOrTesting("wiki hook-post-receive is not supported")
		return
	}

	ownerName := ctx.PathParam("owner")
	repoName := ctx.PathParam("repo")
	repo := loadRepository(ctx, ownerName, repoName)
	if ctx.Written() {
		return
	}
	// now, repo can't be nil

	// first, collect updates and sync branches
	updates := hookPostReceiveCollectPushUpdates(opts, repo)
	if !hookPostReceiveSyncDatabaseBranches(ctx, opts, repo, updates) {
		return
	}
	hookPostReceiveSyncRepoDefaultBranch(ctx, opts, repo)

	// handle pull request merging, a pull request action should push at least 1 commit
	if opts.PushTrigger == repo_module.PushTriggerPRMergeToBase {
		if !hookPostReceiveHandlePullRequestMerging(ctx, opts, updates) {
			return
		}
	}

	if !hookPostReceiveUpdateRepoByOptions(ctx, opts, repo) {
		return
	}

	// push async updates
	if err := repo_service.PushUpdates(updates...); err != nil {
		ctx.PrivateError(http.StatusInternalServerError, err, "failed to push updates")
		return
	}

	hookPostReceiveRespondWithTrailer(ctx, opts, repo)
}

func hookPostReceiveUpdateRepoByOptions(ctx *gitea_context.PrivateContext, opts *private.HookOptions, repo *repo_model.Repository) bool {
	isPrivate := opts.GitPushOptions.Bool(private.GitPushOptionRepoPrivate)
	isTemplate := opts.GitPushOptions.Bool(private.GitPushOptionRepoTemplate)
	// Handle Push Options
	if isPrivate.Has() || isTemplate.Has() {
		pusher, err := loadContextCacheUser(ctx, opts.UserID)
		if err != nil {
			ctx.PrivateError(http.StatusInternalServerError, err, "failed to load pusher user")
			return false
		}
		perm, err := access_model.GetDoerRepoPermission(ctx, repo, pusher)
		if err != nil {
			ctx.PrivateError(http.StatusInternalServerError, err, "failed to load doer repo permission")
			return false
		}
		if !perm.IsOwner() && !perm.IsAdmin() {
			ctx.PrivateError(http.StatusNotFound, nil, "permission denied")
			return false
		}

		// FIXME: these options are not quite right, for example: changing visibility should do more works than just setting the is_private flag
		// These options should only be used for "push-to-create"
		if isPrivate.Has() && repo.IsPrivate != isPrivate.Value() {
			// TODO: it needs to do more work
			repo.IsPrivate = isPrivate.Value()
			if err = repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "is_private"); err != nil {
				log.Error("failed to update repo is_private: %v", err)
			}
		}
		if isTemplate.Has() && repo.IsTemplate != isTemplate.Value() {
			repo.IsTemplate = isTemplate.Value()
			if err = repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "is_template"); err != nil {
				log.Error("failed to update repo is_template: %v", err)
			}
		}
	}
	return true
}

func hookPostReceiveRespondWithTrailer(ctx *gitea_context.PrivateContext, opts *private.HookOptions, repo *repo_model.Repository) {
	results := make([]private.HookPostReceiveBranchResult, 0, len(opts.OldCommitIDs))
	baseRepo := repo
	if repo.IsFork {
		if err := repo.GetBaseRepo(ctx); err != nil {
			ctx.PrivateError(http.StatusInternalServerError, err, "failed to load base repo")
			return
		}
		if repo.BaseRepo.AllowsPulls(ctx) {
			baseRepo = repo.BaseRepo
		}
	}

	if !baseRepo.AllowsPulls(ctx) {
		// We can stop there's no need to go any further
		ctx.JSON(http.StatusOK, private.HookPostReceiveResult{})
		return
	}

	// Now handle the pull request notification trailers
	for i := range opts.OldCommitIDs {
		refFullName := opts.RefFullNames[i]
		newCommitID := opts.NewCommitIDs[i]

		// If we've pushed a branch (and not deleted it)
		if !git.IsEmptyCommitID(newCommitID) && refFullName.IsBranch() {
			branch := refFullName.BranchName()

			if branch == baseRepo.DefaultBranch && !repo.IsFork {
				// If our branch is the default branch of an unforked repo - there's no PR to create or refer to
				results = append(results, private.HookPostReceiveBranchResult{})
				continue
			}

			pr, err := issues_model.GetUnmergedPullRequest(ctx, repo.ID, baseRepo.ID, branch, baseRepo.DefaultBranch, issues_model.PullRequestFlowGithub)
			if err != nil && !errors.Is(err, util.ErrNotExist) {
				ctx.PrivateError(http.StatusInternalServerError, err, "failed to get active PR for branch "+branch)
				return
			}
			if pr == nil {
				results = append(results, private.HookPostReceiveBranchResult{
					Message: setting.Git.PullRequestPushMessage && baseRepo.AllowsPulls(ctx),
					Create:  true,
					Branch:  branch,
					URL:     fmt.Sprintf("%s/pulls/new/%s", repo.HTMLURL(), util.PathEscapeSegments(branch)),
				})
			} else {
				results = append(results, private.HookPostReceiveBranchResult{
					Message: setting.Git.PullRequestPushMessage && baseRepo.AllowsPulls(ctx),
					Create:  false,
					Branch:  branch,
					URL:     fmt.Sprintf("%s/pulls/%d", baseRepo.HTMLURL(), pr.Index),
				})
			}
		}
	}
	ctx.JSON(http.StatusOK, private.HookPostReceiveResult{Results: results})
}

func loadContextCacheUser(ctx context.Context, id int64) (*user_model.User, error) {
	return cache.GetWithContextCache(ctx, cachegroup.User, id, user_model.GetUserByID)
}

// hookPostReceiveHandlePullRequestMerging handle pull request merging, a pull request action should push at least 1 commit
func hookPostReceiveHandlePullRequestMerging(ctx *gitea_context.PrivateContext, opts *private.HookOptions, updates []*repo_module.PushUpdateOptions) bool {
	if len(updates) == 0 {
		err := fmt.Errorf("Pushing a merged PR (pr:%d) no commits pushed ", opts.PullRequestID)
		ctx.PrivateError(http.StatusInternalServerError, err, "no push update")
		return false
	}

	pr, err := issues_model.GetPullRequestByID(ctx, opts.PullRequestID)
	if err != nil {
		ctx.PrivateError(http.StatusInternalServerError, err, "failed to load pull request")
		return false
	}

	pusher, err := loadContextCacheUser(ctx, opts.UserID)
	if err != nil {
		ctx.PrivateError(http.StatusInternalServerError, err, "failed to load pusher user")
		return false
	}

	// FIXME: Maybe we need a `PullRequestStatusMerged` status for PRs that are merged, currently we use the previous status
	// here to keep it as before, that maybe PullRequestStatusMergeable
	_, err = pull_service.SetMerged(ctx, pr, updates[len(updates)-1].NewCommitID, timeutil.TimeStampNow(), pusher, pr.Status)
	if err != nil {
		ctx.PrivateError(http.StatusInternalServerError, err, "failed to set pr to merged")
		return false
	}
	return true
}

func hookPostReceiveSyncRepoDefaultBranch(ctx *gitea_context.PrivateContext, opts *private.HookOptions, repo *repo_model.Repository) {
	hasBranch := false
	for _, refFullName := range opts.RefFullNames {
		if hasBranch = refFullName.IsBranch(); hasBranch {
			break
		}
	}
	if !hasBranch {
		return
	}
	gitRepo, err := gitrepo.RepositoryFromRequestContextOrOpen(ctx, repo)
	if err != nil {
		log.Error("failed to open git repo: %v", err)
		return
	}

	// if default branch doesn't exist, try to guess one from existing git repo
	_, err = gitRepo.GetBranchCommitID(repo.DefaultBranch)
	if errors.Is(err, util.ErrNotExist) {
		for _, guessBranchName := range []string{"main", "master"} {
			if _, err = gitRepo.GetBranchCommitID(guessBranchName); err == nil {
				repo.DefaultBranch = guessBranchName
				err = repo_model.UpdateDefaultBranch(ctx, repo)
				if err != nil {
					log.Error("failed to update default branch: %v", err)
					return
				}
				break
			}
		}
	}

	// if default branch was pushed, always keep the HEAD ref in sync
	for _, refFullName := range opts.RefFullNames {
		if refFullName.IsBranch() && refFullName.BranchName() == repo.DefaultBranch {
			_ = gitrepo.SetDefaultBranch(ctx, repo, repo.DefaultBranch)
		}
	}
}
