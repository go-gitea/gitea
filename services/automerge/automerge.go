// Copyright 2021 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package automerge

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/queue"
	pull_service "code.gitea.io/gitea/services/pull"
)

// prAutoMergeQueue represents a queue to handle update pull request tests
var prAutoMergeQueue queue.UniqueQueue

// Init runs the task queue to that handles auto merges
func Init() error {
	prAutoMergeQueue = queue.CreateUniqueQueue("pr_auto_merge", handle, "")
	if prAutoMergeQueue == nil {
		return fmt.Errorf("Unable to create pr_auto_merge Queue")
	}
	go graceful.GetManager().RunWithShutdownFns(prAutoMergeQueue.Run)
	return nil
}

// handle passed PR IDs and test the PRs
func handle(data ...queue.Data) []queue.Data {
	for _, d := range data {
		id, _ := strconv.ParseInt(d.(string), 10, 64)
		handlePull(id)
	}
	return nil
}

func add2Queue(pr *models.PullRequest, sha string) {
	if err := prAutoMergeQueue.PushFunc(strconv.FormatInt(pr.ID, 10), func() error {
		log.Trace("Adding pullID: %d to the pull requests patch checking queue with sha %s", pr.ID, sha)
		return nil
	}); err != nil {
		log.Error("Error adding pullID: %d to the pull requests patch checking queue %v", pr.ID, err)
	}
}

// ScheduleAutoMerge if schedule is false and no error, pull can be merged directly
func ScheduleAutoMerge(ctx context.Context, doer *user_model.User, pull *models.PullRequest, style repo_model.MergeStyle, message string) (scheduled bool, err error) {
	lastCommitStatus, err := pull_service.GetPullRequestCommitStatusState(ctx, pull)
	if err != nil {
		return false, err
	}

	// we don't need to schedule
	if lastCommitStatus.IsSuccess() {
		return false, nil
	}

	return true, pull_model.ScheduleAutoMerge(ctx, doer, pull.ID, style, message)
}

// MergeScheduledPullRequest merges a previously scheduled pull request when all checks succeeded
func MergeScheduledPullRequest(ctx context.Context, sha string, repo *repo_model.Repository) error {
	pulls, err := getPullRequestsByHeadSHA(ctx, sha, repo, func(pr *models.PullRequest) bool {
		return !pr.HasMerged && pr.CanAutoMerge()
	})
	if err != nil {
		return err
	}

	for _, pr := range pulls {
		add2Queue(pr, sha)
	}

	return nil
}

func getPullRequestsByHeadSHA(ctx context.Context, sha string, repo *repo_model.Repository, filter func(*models.PullRequest) bool) (map[int64]*models.PullRequest, error) {
	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
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

			prIndex, err := strconv.ParseInt(parts[0], 10, 64)
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
			prs, err := models.GetPullRequestsByHeadBranch(ctx, ref[len(git.BranchPrefix):], repo.ID)
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

func handlePull(pullID int64) {
	stdCtx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(),
		fmt.Sprintf("Handle AutoMerge of PR[%d]", pullID))
	defer finished()

	ctx, committer, err := db.TxContext()
	if err != nil {
		log.Error(err.Error())
		return
	}
	defer committer.Close()
	ctx.WithContext(stdCtx)

	pr, err := models.GetPullRequestByID(ctx, pullID)
	if err != nil {
		log.Error("GetPullRequestByID[%d]: %v", pullID, err)
		return
	}

	// Check if there is a scheduled pr in the db
	exists, scheduledPRM, err := pull_model.GetScheduledMergeByPullID(ctx, pr.ID)
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

	headGitRepo, err := git.OpenRepository(ctx, pr.HeadRepo.RepoPath())
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
		log.Info("Scheduled auto merge pr has unsuccessful status checks [PullID: %d]", pr.ID)
		return
	}

	// Merge if all checks succeeded
	doer, err := user_model.GetUserByIDCtx(ctx, scheduledPRM.DoerID)
	if err != nil {
		log.Error(err.Error())
		return
	}

	perm, err := models.GetUserRepoPermission(ctx, pr.HeadRepo, doer)
	if err != nil {
		log.Error(err.Error())
		return
	}

	if err := pull_service.CheckPullMergable(ctx, doer, &perm, pr, false, false); err != nil {
		if errors.Is(pull_service.ErrUserNotAllowedToMerge, err) {
			log.Debug("pull[%d] not ready for automerge", pr.ID)
			return
		}
		log.Error(err.Error())
		return
	}

	var baseGitRepo *git.Repository
	if pr.BaseRepoID == pr.HeadRepoID {
		baseGitRepo = headGitRepo
	} else {
		if err = pr.LoadBaseRepoCtx(ctx); err != nil {
			log.Error(err.Error())
			return
		}

		baseGitRepo, err = git.OpenRepository(ctx, pr.BaseRepo.RepoPath())
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
