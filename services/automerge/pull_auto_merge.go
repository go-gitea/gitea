// Copyright 2021 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package automerge

import (
	"context"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	pull_service "code.gitea.io/gitea/services/pull"
)

// This package merges a previously scheduled pull request on successful status check.
// It is a separate package to avoid cyclic dependencies.

// MergeScheduledPullRequest merges a previously scheduled pull request when all checks succeeded
func MergeScheduledPullRequest(ctx context.Context, sha string, repo *repo_model.Repository) error {
	pulls, err := getPullRequestsByHeadSHA(ctx, sha, repo, func(pr *models.PullRequest) bool {
		return !pr.HasMerged && pr.CanAutoMerge()
	})
	if err != nil {
		return err
	}

	for _, pr := range pulls {
		go handlePull(pr, sha)
	}

	return nil
}

func getPullRequestsByHeadSHA(ctx context.Context, sha string, repo *repo_model.Repository, filter func(*models.PullRequest) bool) (map[int64]*models.PullRequest, error) {
	gitRepo, err := git.OpenRepositoryCtx(ctx, repo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	refs, err := gitRepo.GetRefsBySha(sha, "")
	if err != nil {
		return nil, err
	}

	pulls := make(map[int64]*models.PullRequest)

	for _, ref := range refs {
		// If the branch starts with "pull/*" we know we're dealing with a fork.
		// In that case, head and base branch are not in the same repo and we need to do some extra work
		// to get the pull request for this branch.
		// Each pull branch starts with refs/pull/ we then go from there to find the index of the pr and then
		// use that to get the pr.
		if strings.HasPrefix(ref, git.PullPrefix) {
			parts := strings.Split(ref[len(git.PullPrefix):], "/")

			// e.g. 'refs/pull/1/head' would be []string{"1", "head"}
			if len(parts) != 2 {
				continue
			}

			prIndex, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				return nil, err
			}

			p, err := models.GetPullRequestByIndexCtx(ctx, repo.ID, prIndex)
			if err != nil {
				// If there is no pull request for this branch, we don't try to merge it.
				if models.IsErrPullRequestNotExist(err) {
					continue
				}
				return nil, err
			}

			if filter(p) {
				pulls[p.ID] = p
			}

		} else if strings.HasPrefix(ref, git.BranchPrefix) {
			prs, err := models.GetPullRequestsByHeadBranch(ctx, ref[len(git.BranchPrefix):], repo)
			if err != nil {
				// If there is no pull request for this branch, we don't try to merge it.
				if models.IsErrPullRequestNotExist(err) {
					continue
				}
				return nil, err
			}
			for _, pr := range prs {
				if filter(pr) {
					pulls[pr.ID] = pr
				}
			}
		}
	}

	return pulls, nil
}

// TODO: queue
func handlePull(pr *models.PullRequest, sha string) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		log.Error(err.Error())
		return
	}
	defer committer.Close()

	// Check if there is a scheduled pr in the db
	exists, scheduledPRM, err := models.GetScheduledPullRequestMergeByPullID(ctx, pr.ID)
	if err != nil {
		log.Error(err.Error())
		return
	}
	if !exists {
		return
	}

	// Get all checks for this pr
	// We get the latest sha commit hash again to handle the case where the check of a previous push
	// did not succeed or was not finished yet.

	if err = pr.LoadHeadRepoCtx(ctx); err != nil {
		log.Error(err.Error())
		return
	}

	headGitRepo, err := git.OpenRepositoryCtx(ctx, pr.HeadRepo.RepoPath())
	if err != nil {
		log.Error(err.Error())
		return
	}
	defer headGitRepo.Close()

	headBranchExist := headGitRepo.IsBranchExist(pr.HeadBranch)

	if pr.HeadRepo == nil || !headBranchExist {
		log.Info("Head branch of auto merge pr does not exist [HeadRepoID: %d, Branch: %s, PRID: %d]", pr.HeadRepoID, pr.HeadBranch, pr.ID)
		return
	}

	// Check if all checks succeeded
	pass, err := pull_service.IsPullCommitStatusPass(ctx, pr)
	if err != nil {
		log.Error(err.Error())
		return
	}
	if !pass {
		log.Info("Scheduled auto merge pr has unsuccessful status checks [PRID: %d, Commit: %s]", pr.ID, sha)
		return
	}

	// TODO: evaluate all protected branch rules

	// Merge if all checks succeeded
	doer, err := user_model.GetUserByIDCtx(ctx, scheduledPRM.DoerID)
	if err != nil {
		log.Error(err.Error())
		return
	}

	var baseGitRepo *git.Repository
	if pr.BaseRepoID == pr.HeadRepoID {
		baseGitRepo = headGitRepo
	} else {
		if err = pr.LoadBaseRepo(); err != nil {
			log.Error(err.Error())
			return
		}

		baseGitRepo, err = git.OpenRepositoryCtx(ctx, pr.BaseRepo.RepoPath())
		if err != nil {
			log.Error(err.Error())
			return
		}
		defer baseGitRepo.Close()
	}

	if err := pull_service.Merge(ctx, pr, doer, baseGitRepo, scheduledPRM.MergeStyle, "", scheduledPRM.Message); err != nil {
		log.Error(err.Error())
		return
	}

	if err := committer.Commit(); err != nil {
		log.Error(err.Error())
	}
}
