// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/private"
	repo_module "gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/audit"
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

func hookPostReceiveSyncDatabaseBranches(ctx *gitea_context.PrivateContext, opts *private.HookOptions, repo *repo_model.Repository, updates []*repo_module.PushUpdateOptions) {
	branchesToSync := make([]*repo_module.PushUpdateOptions, 0, len(updates))
	for _, update := range updates {
		if !update.RefFullName.IsBranch() {
			continue
		}
		if update.IsDelRef() {
			if err := git_model.MarkBranchAsDeleted(ctx, repo.ID, update.RefFullName.BranchName(), update.PusherID); err != nil {
				log.Error("failed to mark branch %s as deleted: %v", update.RefFullName, err)
				continue
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

	gitRepo, err := git.RepositoryFromRequestContextOrOpen(ctx, repo)
	if err != nil {
		log.Error("failed to open git repo: %v", err)
		return
	}

	branchNames := make([]string, 0, len(branchesToSync))
	commitIDs := make([]string, 0, len(branchesToSync))
	for _, update := range branchesToSync {
		branchNames = append(branchNames, update.RefFullName.BranchName())
		commitIDs = append(commitIDs, update.NewCommitID)
	}
	var defaultBranch string
	// in one transaction, so readers never see an empty repo with branches or a non-empty repo without
	err = db.WithTx(ctx, func(ctx context.Context) (err error) {
		if err = repo_service.SyncBranchesToDB(ctx, repo.ID, opts.UserID, gitRepo, branchNames, commitIDs); err != nil {
			return err
		}
		if repo.IsEmpty {
			if defaultBranch, err = hookPostReceiveMarkRepoNotEmpty(ctx, gitRepo, repo, branchNames[0]); err != nil {
				return fmt.Errorf("mark repo as not empty: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		log.Error("failed to sync branch to DB: %v", err)
		return
	}
	if repo.IsEmpty {
		repo.IsEmpty, repo.DefaultBranch = false, defaultBranch
		// the default branch might not be in this push, so sync HEAD here
		if err = git.SetDefaultBranch(ctx, repo, defaultBranch); err != nil {
			log.Error("failed to set default branch: %v", err)
		}
	}
}

// hookPostReceiveMarkRepoNotEmpty returns the default branch of the repo after marking it as not empty
func hookPostReceiveMarkRepoNotEmpty(ctx context.Context, gitRepo *git.Repository, repo *repo_model.Repository, firstPushedBranch string) (string, error) {
	defaultBranch, err := hookPostReceiveFindDefaultBranch(ctx, gitRepo, repo.DefaultBranch)
	if err != nil {
		return "", err
	}
	defaultBranch = util.IfZero(defaultBranch, firstPushedBranch)
	marked, err := repo_model.MarkRepositoryNotEmpty(ctx, repo.ID, defaultBranch)
	if err != nil {
		return "", err
	}
	if marked {
		return defaultBranch, nil
	}
	// marked by others, keep the default branch if it exists in git
	freshRepo, err := repo_model.GetRepositoryByID(ctx, repo.ID)
	if err != nil {
		return "", err
	}
	if _, err = gitRepo.GetBranchCommitID(ctx, freshRepo.DefaultBranch); !errors.Is(err, util.ErrNotExist) {
		return freshRepo.DefaultBranch, err
	}
	freshRepo.DefaultBranch = defaultBranch // the current default branch doesn't exist in git
	return defaultBranch, repo_model.UpdateDefaultBranch(ctx, freshRepo)
}

// HookPostReceive updates services and users
func HookPostReceive(ctx *gitea_context.PrivateContext) {
	opts := web.GetForm[*private.HookOptions](ctx)
	if opts.IsWiki {
		setting.PanicInDevOrTesting("wiki hook-post-receive is not supported")
		return
	}

	// In the post-receive hook, the git repo has received all changes,
	// So even if any error happens, nothing can be stopped or undone, so the errors should be skipped
	repo := ctx.Repo.Repository
	updates := hookPostReceiveCollectPushUpdates(opts, repo)
	hookPostReceiveUpdateRepoByOptions(ctx, opts, repo) // push options are only honored while the repo is still empty
	hookPostReceiveSyncDatabaseBranches(ctx, opts, repo, updates)
	hookPostReceiveSyncRepoDefaultBranch(ctx, opts, repo)

	// handle async updates (e.g.: notification, cache management, repo update time, etc.)
	if err := repo_service.PushUpdates(updates...); err != nil {
		log.Error("failed to push updates: %v", err)
	}

	hookPostReceiveRespondWithTrailer(ctx, opts, repo)
}

func hookPostReceiveUpdateRepoByOptions(ctx *gitea_context.PrivateContext, opts *private.HookOptions, repo *repo_model.Repository) {
	isPrivate := opts.GitPushOptions.Bool(private.GitPushOptionRepoPrivate)
	isTemplate := opts.GitPushOptions.Bool(private.GitPushOptionRepoTemplate)
	// Handle Push Options
	if !isPrivate.Has() && !isTemplate.Has() {
		return
	}

	if !loadContextDoerPermission(ctx, opts.UserID, opts.UserExtDoerData) {
		return
	}

	if !ctx.Repo.Permission.IsAdmin() {
		return
	}

	// Only honor these options while the repo is still empty (the push-to-create
	// case). On a populated repo a bare "git push -o repo.private=..." would
	// silently flip visibility, bypassing the audit log, webhooks and notifications.
	if !repo.IsEmpty {
		return
	}

	// The repo is empty and being initialized by this push, so there is no
	// dependent state (webhooks, notifications, visibility fan-out) to reconcile
	// yet; setting the flags directly is sufficient in this push-to-create case.
	if isPrivate.Has() && repo.IsPrivate != isPrivate.Value() &&
		(isPrivate.Value() || !setting.Repository.ForcePrivate || ctx.Doer.IsAdmin) {
		repo.IsPrivate = isPrivate.Value()
		if err := repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "is_private"); err != nil {
			log.Error("failed to update repo is_private: %v", err)
		} else {
			audit.RecordAs(ctx, ctx.Doer, audit_model.RepositoryVisibility, repo, "visibility", repo.IsPrivate)
		}
	}
	if isTemplate.Has() && repo.IsTemplate != isTemplate.Value() {
		repo.IsTemplate = isTemplate.Value()
		if err := repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "is_template"); err != nil {
			log.Error("failed to update repo is_template: %v", err)
		}
	}
}

func hookPostReceiveRespondWithTrailer(ctx *gitea_context.PrivateContext, opts *private.HookOptions, repo *repo_model.Repository) {
	results := make([]private.HookPostReceiveBranchResult, 0, len(opts.OldCommitIDs))
	baseRepo := repo
	if repo.IsFork {
		if err := repo.GetBaseRepo(ctx); err != nil {
			ctx.PrivateInternalErrorf("failed to load base repo: %v", err)
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
				ctx.PrivateInternalErrorf("failed to get active PR for branch %s: %v", branch, err)
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

func hookPostReceiveSyncRepoDefaultBranch(ctx *gitea_context.PrivateContext, opts *private.HookOptions, repo *repo_model.Repository) {
	if repo.IsEmpty || !slices.ContainsFunc(opts.RefFullNames, git.RefName.IsBranch) {
		return
	}
	gitRepo, err := git.RepositoryFromRequestContextOrOpen(ctx, repo)
	if err != nil {
		log.Error("failed to open git repo: %v", err)
		return
	}

	// if default branch doesn't exist, try to guess one from existing git repo
	defaultBranch, err := hookPostReceiveFindDefaultBranch(ctx, gitRepo, repo.DefaultBranch)
	if err != nil {
		log.Error("failed to find default branch: %v", err)
	}
	defaultBranchChanged := defaultBranch != "" && defaultBranch != repo.DefaultBranch
	if defaultBranchChanged {
		repo.DefaultBranch = defaultBranch
		if err = repo_model.UpdateDefaultBranch(ctx, repo); err != nil {
			log.Error("failed to update default branch: %v", err)
			return
		}
	}

	// if default branch was changed or pushed, always keep the HEAD ref in sync
	if defaultBranchChanged || slices.Contains(opts.RefFullNames, git.RefNameFromBranch(repo.DefaultBranch)) {
		_ = git.SetDefaultBranch(ctx, repo, repo.DefaultBranch)
	}
}

// hookPostReceiveFindDefaultBranch returns the first one of the default branch, main and master that exists in git
func hookPostReceiveFindDefaultBranch(ctx context.Context, gitRepo *git.Repository, defaultBranch string) (string, error) {
	for _, branch := range []string{defaultBranch, "main", "master"} {
		if _, err := gitRepo.GetBranchCommitID(ctx, branch); err == nil {
			return branch, nil
		} else if !errors.Is(err, util.ErrNotExist) {
			return "", err
		}
	}
	return "", nil
}
