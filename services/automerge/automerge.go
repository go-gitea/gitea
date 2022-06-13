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

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
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
		var id int64
		var sha string
		if _, err := fmt.Sscanf(d.(string), "%d_%s", &id, &sha); err != nil {
			log.Error("could not parse data from pr_auto_merge queue (%v): %v", d, err)
			continue
		}
		handlePull(id, sha)
	}
	return nil
}

func addToQueue(pr *issues_model.PullRequest, sha string) {
	if err := prAutoMergeQueue.PushFunc(fmt.Sprintf("%d_%s", pr.ID, sha), func() error {
		log.Trace("Adding pullID: %d to the pull requests patch checking queue with sha %s", pr.ID, sha)
		return nil
	}); err != nil {
		log.Error("Error adding pullID: %d to the pull requests patch checking queue %v", pr.ID, err)
	}
}

// ScheduleAutoMerge if schedule is false and no error, pull can be merged directly
func ScheduleAutoMerge(ctx context.Context, doer *user_model.User, pull *issues_model.PullRequest, style repo_model.MergeStyle, message string) (scheduled bool, err error) {
	err = db.WithTx(func(ctx context.Context) error {
		lastCommitStatus, err := pull_service.GetPullRequestCommitStatusState(ctx, pull)
		if err != nil {
			return err
		}

		// we don't need to schedule
		if lastCommitStatus.IsSuccess() {
			return nil
		}

		if err := pull_model.ScheduleAutoMerge(ctx, doer, pull.ID, style, message); err != nil {
			return err
		}
		scheduled = true

		_, err = issues_model.CreateAutoMergeComment(ctx, issues_model.CommentTypePRScheduledToAutoMerge, pull, doer)
		return err
	}, ctx)
	return
}

// RemoveScheduledAutoMerge cancels a previously scheduled pull request
func RemoveScheduledAutoMerge(ctx context.Context, doer *user_model.User, pull *issues_model.PullRequest) error {
	return db.WithTx(func(ctx context.Context) error {
		if err := pull_model.DeleteScheduledAutoMerge(ctx, pull.ID); err != nil {
			return err
		}

		_, err := issues_model.CreateAutoMergeComment(ctx, issues_model.CommentTypePRUnScheduledToAutoMerge, pull, doer)
		return err
	}, ctx)
}

// MergeScheduledPullRequest merges a previously scheduled pull request when all checks succeeded
func MergeScheduledPullRequest(ctx context.Context, sha string, repo *repo_model.Repository) error {
	pulls, err := getPullRequestsByHeadSHA(ctx, sha, repo, func(pr *issues_model.PullRequest) bool {
		return !pr.HasMerged && pr.CanAutoMerge()
	})
	if err != nil {
		return err
	}

	for _, pr := range pulls {
		addToQueue(pr, sha)
	}

	return nil
}

func getPullRequestsByHeadSHA(ctx context.Context, sha string, repo *repo_model.Repository, filter func(*issues_model.PullRequest) bool) (map[int64]*issues_model.PullRequest, error) {
	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	refs, err := gitRepo.GetRefsBySha(sha, "")
	if err != nil {
		return nil, err
	}

	pulls := make(map[int64]*issues_model.PullRequest)

	for _, ref := range refs {
		// Each pull branch starts with refs/pull/ we then go from there to find the index of the pr and then
		// use that to get the pr.
		if strings.HasPrefix(ref, git.PullPrefix) {
			parts := strings.Split(ref[len(git.PullPrefix):], "/")

			// e.g. 'refs/pull/1/head' would be []string{"1", "head"}
			if len(parts) != 2 {
				log.Error("getPullRequestsByHeadSHA found broken pull ref [%s] on repo [%-v]", ref, repo)
				continue
			}

			prIndex, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				log.Error("getPullRequestsByHeadSHA found broken pull ref [%s] on repo [%-v]", ref, repo)
				continue
			}

			p, err := issues_model.GetPullRequestByIndex(ctx, repo.ID, prIndex)
			if err != nil {
				// If there is no pull request for this branch, we don't try to merge it.
				if issues_model.IsErrPullRequestNotExist(err) {
					continue
				}
				return nil, err
			}

			if filter(p) {
				pulls[p.ID] = p
			}
		}
	}

	return pulls, nil
}

func handlePull(pullID int64, sha string) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(),
		fmt.Sprintf("Handle AutoMerge of pull[%d] with sha[%s]", pullID, sha))
	defer finished()

	pr, err := issues_model.GetPullRequestByID(ctx, pullID)
	if err != nil {
		log.Error("GetPullRequestByID[%d]: %v", pullID, err)
		return
	}

	// Check if there is a scheduled pr in the db
	exists, scheduledPRM, err := pull_model.GetScheduledMergeByPullID(ctx, pr.ID)
	if err != nil {
		log.Error("pull[%d] GetScheduledMergeByPullID: %v", pr.ID, err)
		return
	}
	if !exists {
		return
	}

	// Get all checks for this pr
	// We get the latest sha commit hash again to handle the case where the check of a previous push
	// did not succeed or was not finished yet.

	if err = pr.LoadHeadRepoCtx(ctx); err != nil {
		log.Error("pull[%d] LoadHeadRepoCtx: %v", pr.ID, err)
		return
	}

	headGitRepo, err := git.OpenRepository(ctx, pr.HeadRepo.RepoPath())
	if err != nil {
		log.Error("OpenRepository: %v", err)
		return
	}
	defer headGitRepo.Close()

	headBranchExist := headGitRepo.IsBranchExist(pr.HeadBranch)

	if pr.HeadRepo == nil || !headBranchExist {
		log.Warn("Head branch of auto merge pr does not exist [HeadRepoID: %d, Branch: %s, PR ID: %d]", pr.HeadRepoID, pr.HeadBranch, pr.ID)
		return
	}

	// Check if all checks succeeded
	pass, err := pull_service.IsPullCommitStatusPass(ctx, pr)
	if err != nil {
		log.Error("IsPullCommitStatusPass: %v", err)
		return
	}
	if !pass {
		log.Info("Scheduled auto merge pr has unsuccessful status checks [PullID: %d]", pr.ID)
		return
	}

	// Merge if all checks succeeded
	doer, err := user_model.GetUserByIDCtx(ctx, scheduledPRM.DoerID)
	if err != nil {
		log.Error("GetUserByIDCtx: %v", err)
		return
	}

	perm, err := access_model.GetUserRepoPermission(ctx, pr.HeadRepo, doer)
	if err != nil {
		log.Error("GetUserRepoPermission: %v", err)
		return
	}

	if err := pull_service.CheckPullMergable(ctx, doer, &perm, pr, false, false); err != nil {
		if errors.Is(pull_service.ErrUserNotAllowedToMerge, err) {
			log.Info("PR %d was scheduled to automerge by an unauthorized user", pr.ID)
			return
		}
		log.Error("pull[%d] CheckPullMergable: %v", pr.ID, err)
		return
	}

	var baseGitRepo *git.Repository
	if pr.BaseRepoID == pr.HeadRepoID {
		baseGitRepo = headGitRepo
	} else {
		if err = pr.LoadBaseRepoCtx(ctx); err != nil {
			log.Error("LoadBaseRepoCtx: %v", err)
			return
		}

		baseGitRepo, err = git.OpenRepository(ctx, pr.BaseRepo.RepoPath())
		if err != nil {
			log.Error("OpenRepository: %v", err)
			return
		}
		defer baseGitRepo.Close()
	}

	if err := pull_service.Merge(ctx, pr, doer, baseGitRepo, scheduledPRM.MergeStyle, "", scheduledPRM.Message); err != nil {
		log.Error("pull_service.Merge: %v", err)
		return
	}
}
