// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"context"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	timeutil "code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	gitea_context "code.gitea.io/gitea/services/context"
	pull_service "code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"
)

// HookPostReceive updates services and users
func HookPostReceive(ctx *gitea_context.PrivateContext) {
	opts := web.GetForm(ctx).(*private.HookOptions)

	// We don't rely on RepoAssignment here because:
	// a) we don't need the git repo in this function
	//    OUT OF DATE: we do need the git repo to sync the branch to the db now.
	// b) our update function will likely change the repository in the db so we will need to refresh it
	// c) we don't always need the repo

	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")

	// defer getting the repository at this point - as we should only retrieve it if we're going to call update
	var (
		repo    *repo_model.Repository
		gitRepo *git.Repository
	)
	defer gitRepo.Close() // it's safe to call Close on a nil pointer

	updates := make([]*repo_module.PushUpdateOptions, 0, len(opts.OldCommitIDs))
	wasEmpty := false

	for i := range opts.OldCommitIDs {
		refFullName := opts.RefFullNames[i]

		// Only trigger activity updates for changes to branches or
		// tags.  Updates to other refs (eg, refs/notes, refs/changes,
		// or other less-standard refs spaces are ignored since there
		// may be a very large number of them).
		if refFullName.IsBranch() || refFullName.IsTag() {
			if repo == nil {
				repo = loadRepository(ctx, ownerName, repoName)
				if ctx.Written() {
					// Error handled in loadRepository
					return
				}
				wasEmpty = repo.IsEmpty
			}

			option := &repo_module.PushUpdateOptions{
				RefFullName:  refFullName,
				OldCommitID:  opts.OldCommitIDs[i],
				NewCommitID:  opts.NewCommitIDs[i],
				PusherID:     opts.UserID,
				PusherName:   opts.UserName,
				RepoUserName: ownerName,
				RepoName:     repoName,
			}
			updates = append(updates, option)
			if repo.IsEmpty && (refFullName.BranchName() == "master" || refFullName.BranchName() == "main") {
				// put the master/main branch first
				// FIXME: It doesn't always work, since the master/main branch may not be the first batch of updates.
				//        If the user pushes many branches at once, the Git hook will call the internal API in batches, rather than all at once.
				//        See https://github.com/go-gitea/gitea/blob/cb52b17f92e2d2293f7c003649743464492bca48/cmd/hook.go#L27
				//        If the user executes `git push origin --all` and pushes more than 30 branches, the master/main may not be the default branch.
				copy(updates[1:], updates)
				updates[0] = option
			}
		}
	}

	if repo != nil && len(updates) > 0 {
		branchesToSync := make([]*repo_module.PushUpdateOptions, 0, len(updates))
		for _, update := range updates {
			if !update.RefFullName.IsBranch() {
				continue
			}
			if repo == nil {
				repo = loadRepository(ctx, ownerName, repoName)
				if ctx.Written() {
					return
				}
				wasEmpty = repo.IsEmpty
			}

			if update.IsDelRef() {
				if err := git_model.AddDeletedBranch(ctx, repo.ID, update.RefFullName.BranchName(), update.PusherID); err != nil {
					log.Error("Failed to add deleted branch: %s/%s Error: %v", ownerName, repoName, err)
					ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
						Err: fmt.Sprintf("Failed to add deleted branch: %s/%s Error: %v", ownerName, repoName, err),
					})
					return
				}
			} else {
				branchesToSync = append(branchesToSync, update)

				// TODO: should we return the error and return the error when pushing? Currently it will log the error and not prevent the pushing
				pull_service.UpdatePullsRefs(ctx, repo, update)
			}
		}
		if len(branchesToSync) > 0 {
			var err error
			gitRepo, err = gitrepo.OpenRepository(ctx, repo)
			if err != nil {
				log.Error("Failed to open repository: %s/%s Error: %v", ownerName, repoName, err)
				ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
					Err: fmt.Sprintf("Failed to open repository: %s/%s Error: %v", ownerName, repoName, err),
				})
				return
			}

			var (
				branchNames = make([]string, 0, len(branchesToSync))
				commitIDs   = make([]string, 0, len(branchesToSync))
			)
			for _, update := range branchesToSync {
				branchNames = append(branchNames, update.RefFullName.BranchName())
				commitIDs = append(commitIDs, update.NewCommitID)
			}

			if err := repo_service.SyncBranchesToDB(ctx, repo.ID, opts.UserID, branchNames, commitIDs, gitRepo.GetCommit); err != nil {
				ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
					Err: fmt.Sprintf("Failed to sync branch to DB in repository: %s/%s Error: %v", ownerName, repoName, err),
				})
				return
			}
		}

		if err := repo_service.PushUpdates(updates); err != nil {
			log.Error("Failed to Update: %s/%s Total Updates: %d", ownerName, repoName, len(updates))
			for i, update := range updates {
				log.Error("Failed to Update: %s/%s Update: %d/%d: Branch: %s", ownerName, repoName, i, len(updates), update.RefFullName.BranchName())
			}
			log.Error("Failed to Update: %s/%s Error: %v", ownerName, repoName, err)

			ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
				Err: fmt.Sprintf("Failed to Update: %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}
	}

	// handle pull request merging, a pull request action should push at least 1 commit
	if opts.PushTrigger == repo_module.PushTriggerPRMergeToBase {
		handlePullRequestMerging(ctx, opts, ownerName, repoName, updates)
		if ctx.Written() {
			return
		}
	}

	isPrivate := opts.GitPushOptions.Bool(private.GitPushOptionRepoPrivate)
	isTemplate := opts.GitPushOptions.Bool(private.GitPushOptionRepoTemplate)
	// Handle Push Options
	if isPrivate.Has() || isTemplate.Has() {
		// load the repository
		if repo == nil {
			repo = loadRepository(ctx, ownerName, repoName)
			if ctx.Written() {
				// Error handled in loadRepository
				return
			}
			wasEmpty = repo.IsEmpty
		}

		pusher, err := loadContextCacheUser(ctx, opts.UserID)
		if err != nil {
			log.Error("Failed to Update: %s/%s Error: %v", ownerName, repoName, err)
			ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
				Err: fmt.Sprintf("Failed to Update: %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}
		perm, err := access_model.GetUserRepoPermission(ctx, repo, pusher)
		if err != nil {
			log.Error("Failed to Update: %s/%s Error: %v", ownerName, repoName, err)
			ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
				Err: fmt.Sprintf("Failed to Update: %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}
		if !perm.IsOwner() && !perm.IsAdmin() {
			ctx.JSON(http.StatusNotFound, private.HookPostReceiveResult{
				Err: "Permissions denied",
			})
			return
		}

		cols := make([]string, 0, len(opts.GitPushOptions))

		if isPrivate.Has() {
			repo.IsPrivate = isPrivate.Value()
			cols = append(cols, "is_private")
		}

		if isTemplate.Has() {
			repo.IsTemplate = isTemplate.Value()
			cols = append(cols, "is_template")
		}

		if len(cols) > 0 {
			if err := repo_model.UpdateRepositoryCols(ctx, repo, cols...); err != nil {
				log.Error("Failed to Update: %s/%s Error: %v", ownerName, repoName, err)
				ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
					Err: fmt.Sprintf("Failed to Update: %s/%s Error: %v", ownerName, repoName, err),
				})
				return
			}
		}
	}

	results := make([]private.HookPostReceiveBranchResult, 0, len(opts.OldCommitIDs))

	// We have to reload the repo in case its state is changed above
	repo = nil
	var baseRepo *repo_model.Repository

	// Now handle the pull request notification trailers
	for i := range opts.OldCommitIDs {
		refFullName := opts.RefFullNames[i]
		newCommitID := opts.NewCommitIDs[i]

		// If we've pushed a branch (and not deleted it)
		if !git.IsEmptyCommitID(newCommitID) && refFullName.IsBranch() {
			// First ensure we have the repository loaded, we're allowed pulls requests and we can get the base repo
			if repo == nil {
				repo = loadRepository(ctx, ownerName, repoName)
				if ctx.Written() {
					return
				}

				baseRepo = repo

				if repo.IsFork {
					if err := repo.GetBaseRepo(ctx); err != nil {
						log.Error("Failed to get Base Repository of Forked repository: %-v Error: %v", repo, err)
						ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
							Err:          fmt.Sprintf("Failed to get Base Repository of Forked repository: %-v Error: %v", repo, err),
							RepoWasEmpty: wasEmpty,
						})
						return
					}
					if repo.BaseRepo.AllowsPulls(ctx) {
						baseRepo = repo.BaseRepo
					}
				}

				if !baseRepo.AllowsPulls(ctx) {
					// We can stop there's no need to go any further
					ctx.JSON(http.StatusOK, private.HookPostReceiveResult{
						RepoWasEmpty: wasEmpty,
					})
					return
				}
			}

			branch := refFullName.BranchName()

			// If our branch is the default branch of an unforked repo - there's no PR to create or refer to
			if !repo.IsFork && branch == baseRepo.DefaultBranch {
				results = append(results, private.HookPostReceiveBranchResult{})
				continue
			}

			pr, err := issues_model.GetUnmergedPullRequest(ctx, repo.ID, baseRepo.ID, branch, baseRepo.DefaultBranch, issues_model.PullRequestFlowGithub)
			if err != nil && !issues_model.IsErrPullRequestNotExist(err) {
				log.Error("Failed to get active PR in: %-v Branch: %s to: %-v Branch: %s Error: %v", repo, branch, baseRepo, baseRepo.DefaultBranch, err)
				ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
					Err: fmt.Sprintf(
						"Failed to get active PR in: %-v Branch: %s to: %-v Branch: %s Error: %v", repo, branch, baseRepo, baseRepo.DefaultBranch, err),
					RepoWasEmpty: wasEmpty,
				})
				return
			}

			if pr == nil {
				if repo.IsFork {
					branch = fmt.Sprintf("%s:%s", repo.OwnerName, branch)
				}
				results = append(results, private.HookPostReceiveBranchResult{
					Message: setting.Git.PullRequestPushMessage && baseRepo.AllowsPulls(ctx),
					Create:  true,
					Branch:  branch,
					URL:     fmt.Sprintf("%s/compare/%s...%s", baseRepo.HTMLURL(), util.PathEscapeSegments(baseRepo.DefaultBranch), util.PathEscapeSegments(branch)),
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
	ctx.JSON(http.StatusOK, private.HookPostReceiveResult{
		Results:      results,
		RepoWasEmpty: wasEmpty,
	})
}

func loadContextCacheUser(ctx context.Context, id int64) (*user_model.User, error) {
	return cache.GetWithContextCache(ctx, "hook_post_receive_user", id, func() (*user_model.User, error) {
		return user_model.GetUserByID(ctx, id)
	})
}

// handlePullRequestMerging handle pull request merging, a pull request action should push at least 1 commit
func handlePullRequestMerging(ctx *gitea_context.PrivateContext, opts *private.HookOptions, ownerName, repoName string, updates []*repo_module.PushUpdateOptions) {
	if len(updates) == 0 {
		ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
			Err: fmt.Sprintf("Pushing a merged PR (pr:%d) no commits pushed ", opts.PullRequestID),
		})
		return
	}

	pr, err := issues_model.GetPullRequestByID(ctx, opts.PullRequestID)
	if err != nil {
		log.Error("GetPullRequestByID[%d]: %v", opts.PullRequestID, err)
		ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{Err: "GetPullRequestByID failed"})
		return
	}

	pusher, err := loadContextCacheUser(ctx, opts.UserID)
	if err != nil {
		log.Error("Failed to Update: %s/%s Error: %v", ownerName, repoName, err)
		ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{Err: "Load pusher user failed"})
		return
	}

	pr.MergedCommitID = updates[len(updates)-1].NewCommitID
	pr.MergedUnix = timeutil.TimeStampNow()
	pr.Merger = pusher
	pr.MergerID = pusher.ID
	err = db.WithTx(ctx, func(ctx context.Context) error {
		// Removing an auto merge pull and ignore if not exist
		if err := pull_model.DeleteScheduledAutoMerge(ctx, pr.ID); err != nil && !db.IsErrNotExist(err) {
			return fmt.Errorf("DeleteScheduledAutoMerge[%d]: %v", opts.PullRequestID, err)
		}
		if _, err := pr.SetMerged(ctx); err != nil {
			return fmt.Errorf("SetMerged failed: %s/%s Error: %v", ownerName, repoName, err)
		}
		return nil
	})
	if err != nil {
		log.Error("Failed to update PR to merged: %v", err)
		ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{Err: "Failed to update PR to merged"})
	}
}
