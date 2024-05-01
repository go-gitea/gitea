// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"fmt"
	"net/http"

	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
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
	var repo *repo_model.Repository
	updates := make([]*repo_module.PushUpdateOptions, 0, len(opts.OldCommitIDs))
	wasEmpty := false

	// generate updates and put the master/main branch first
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

	// sync branches to the database, if failed return error to keep branches consistent between disk and database
	if repo != nil && len(updates) > 0 {
		syncBranches(ctx, updates, repo, opts.UserID)
		if ctx.Written() {
			return
		}
	}

	// Handle possible Push Options
	handlePushOptions(ctx, opts, repo, ownerName, repoName)
	if ctx.Written() {
		return
	}

	// push updates to a queue so some notificactions can be handled async
	if len(updates) > 0 {
		if err := repo_service.PushUpdates(updates); err != nil {
			log.Error("Failed to Update: %s/%s Total Updates: %d, Error: %v", ownerName, repoName, len(updates), err)
			for i, update := range updates {
				log.Error("Failed to Update: %s/%s Update: %d/%d: Branch: %s", ownerName, repoName, i, len(updates), update.RefFullName.BranchName())
			}

			ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
				Err: fmt.Sprintf("Failed to Update: %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}
	}

	// generate branch results for end user. i.e. Displaying a link to create a PR
	results := generateBranchResults(ctx, opts, repo, ownerName, repoName, wasEmpty)
	if ctx.Written() {
		return
	}

	ctx.JSON(http.StatusOK, private.HookPostReceiveResult{
		Results:      results,
		RepoWasEmpty: wasEmpty,
	})
}

func syncBranches(ctx *gitea_context.PrivateContext, updates []*repo_module.PushUpdateOptions, repo *repo_model.Repository, pusherID int64) {
	branchesToSync := make([]*repo_module.PushUpdateOptions, 0, len(updates))
	for _, update := range updates {
		if !update.RefFullName.IsBranch() {
			continue
		}

		if update.IsDelRef() {
			if err := git_model.AddDeletedBranch(ctx, repo.ID, update.RefFullName.BranchName(), update.PusherID); err != nil {
				log.Error("Failed to add deleted branch: %s/%s Error: %v", repo.OwnerName, repo.Name, err)
				ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
					Err: fmt.Sprintf("Failed to add deleted branch: %s/%s Error: %v", repo.OwnerName, repo.Name, err),
				})
				return
			}
		} else {
			branchesToSync = append(branchesToSync, update)

			// TODO: should we return the error and return the error when pushing? Currently it will log the error and not prevent the pushing
			pull_service.UpdatePullsRefs(ctx, repo, update)
		}
	}
	if len(branchesToSync) == 0 {
		return
	}

	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		log.Error("Failed to open repository: %s/%s Error: %v", repo.OwnerName, repo.Name, err)
		ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
			Err: fmt.Sprintf("Failed to open repository: %s/%s Error: %v", repo.OwnerName, repo.Name, err),
		})
		return
	}
	defer gitRepo.Close()

	var (
		branchNames = make([]string, 0, len(branchesToSync))
		commitIDs   = make([]string, 0, len(branchesToSync))
	)
	for _, update := range branchesToSync {
		branchNames = append(branchNames, update.RefFullName.BranchName())
		commitIDs = append(commitIDs, update.NewCommitID)
	}

	if err := repo_service.SyncBranchesToDB(ctx, repo.ID, pusherID, branchNames, commitIDs, gitRepo.GetCommit); err != nil {
		ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
			Err: fmt.Sprintf("Failed to sync branch to DB in repository: %s/%s Error: %v", repo.OwnerName, repo.Name, err),
		})
		return
	}
}

func handlePushOptions(ctx *gitea_context.PrivateContext, opts *private.HookOptions, repo *repo_model.Repository, ownerName, repoName string) {
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
		}

		pusher, err := user_model.GetUserByID(ctx, opts.UserID)
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
}

func generateBranchResults(ctx *gitea_context.PrivateContext, opts *private.HookOptions, repo *repo_model.Repository, ownerName, repoName string, wasEmpty bool) []private.HookPostReceiveBranchResult {
	results := make([]private.HookPostReceiveBranchResult, 0, len(opts.OldCommitIDs))
	var baseRepo *repo_model.Repository

	// Now handle the pull request notification trailers
	for i := range opts.OldCommitIDs {
		refFullName := opts.RefFullNames[i]
		newCommitID := opts.NewCommitIDs[i]

		// If we've pushed a branch (and not deleted it)
		if !refFullName.IsBranch() || git.IsEmptyCommitID(newCommitID) {
			continue
		}

		// First ensure we have the repository loaded, we're allowed pulls requests and we can get the base repo
		if repo == nil {
			repo = loadRepository(ctx, ownerName, repoName)
			if ctx.Written() {
				return nil
			}

			baseRepo = repo

			if repo.IsFork {
				if err := repo.GetBaseRepo(ctx); err != nil {
					log.Error("Failed to get Base Repository of Forked repository: %-v Error: %v", repo, err)
					ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
						Err:          fmt.Sprintf("Failed to get Base Repository of Forked repository: %-v Error: %v", repo, err),
						RepoWasEmpty: wasEmpty,
					})
					return nil
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
				return nil
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
			return nil
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

	return results
}
