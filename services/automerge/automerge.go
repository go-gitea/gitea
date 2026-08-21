// Copyright 2021 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package automerge

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/process"
	"gitea.dev/modules/queue"
	"gitea.dev/modules/util"
	"gitea.dev/services/automergequeue"
	notify_service "gitea.dev/services/notify"
	pull_service "gitea.dev/services/pull"
	repo_service "gitea.dev/services/repository"
)

// Init runs the task queue to that handles auto merges
func Init() error {
	notify_service.RegisterNotifier(NewNotifier())

	automergequeue.AutoMergeQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "pr_auto_merge", handler)
	if automergequeue.AutoMergeQueue == nil {
		return errors.New("unable to create pr_auto_merge queue")
	}
	go graceful.GetManager().RunWithCancel(automergequeue.AutoMergeQueue)
	return nil
}

func handler(items ...automergequeue.AutoMergeItem) []automergequeue.AutoMergeItem {
	for _, item := range items {
		handleAutoMergeItem(item)
	}
	return nil
}

// ScheduleAutoMerge if schedule is false and no error, pull can be merged directly
func ScheduleAutoMerge(ctx context.Context, doer *user_model.User, pull *issues_model.PullRequest, style repo_model.MergeStyle, message string, deleteBranchAfterMerge bool) (scheduled bool, err error) {
	err = db.WithTx(ctx, func(ctx context.Context) error {
		if err := pull_model.ScheduleAutoMerge(ctx, doer, pull.ID, style, message, deleteBranchAfterMerge); err != nil {
			return err
		}
		_, err = issues_model.CreateAutoMergeComment(ctx, issues_model.CommentTypePRScheduledToAutoMerge, pull, doer)
		return err
	})
	// Old code made "scheduled" to be true after "ScheduleAutoMerge", but it's not right:
	// If the transaction rolls back, then the pull request is not scheduled to auto merge.
	// So we should only set "scheduled" to true if there is no error.
	scheduled = err == nil
	if scheduled {
		log.Trace("Pull request [%d] scheduled for auto merge with style [%s] and message [%s]", pull.ID, style, message)
		automergequeue.StartPRCheckAndAutoMerge(pull)
	}
	return scheduled, err
}

// RemoveScheduledAutoMerge cancels a previously scheduled pull request
func RemoveScheduledAutoMerge(ctx context.Context, doer *user_model.User, pull *issues_model.PullRequest) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := pull_model.DeleteScheduledAutoMerge(ctx, pull.ID); err != nil {
			return err
		}

		_, err := issues_model.CreateAutoMergeComment(ctx, issues_model.CommentTypePRUnScheduledToAutoMerge, pull, doer)
		return err
	})
}

func handleAutoMergeItem(item automergequeue.AutoMergeItem) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("AutoMerge: %s", string(item)))
	defer finished()

	parsed := item.Parse()
	if parsed.PullID != 0 {
		handlePullRequestAutoMergeByPullID(ctx, parsed.PullID)
	} else if parsed.RepoID != 0 {
		handleAutoMergeByRepoCommit(ctx, parsed.RepoID, parsed.CommitID)
	} else {
		log.Error("unsupported automerge item: %q", item)
	}
}

// handleRepoCommitAutoMerge queues an automerge check for every pull request whose head is the given commit
func handleAutoMergeByRepoCommit(ctx context.Context, repoID int64, commitID string) {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		log.Error("GetRepositoryByID[%d]: %v", repoID, err)
		return
	}

	pulls, err := enumPullRequestsByHeadCommitID(ctx, commitID, repo, func(pr *issues_model.PullRequest) bool {
		return !pr.HasMerged && pr.IsStatusMergeable()
	})
	if err != nil {
		log.Error("enumPullRequestsByHeadCommitID[%-v, %s]: %v", repo, commitID, err)
		return
	}
	for _, pr := range pulls {
		handlePullRequestAutoMerge(ctx, pr)
	}
}

func enumPullRequestsByHeadCommitID(ctx context.Context, commitID string, repo *repo_model.Repository, filter func(*issues_model.PullRequest) bool) (map[int64]*issues_model.PullRequest, error) {
	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	// Pull ref is something like "refs/pull/1/head"
	refs, err := gitRepo.GetRefsBySha(ctx, commitID, git.PullPrefix)
	if err != nil {
		return nil, err
	}

	pulls := make(map[int64]*issues_model.PullRequest)
	for _, ref := range refs {
		refPart, ok := strings.CutPrefix(ref, "refs/heads/")
		if !ok {
			continue
		}
		parts := strings.Split(refPart, "/") // the parts are from "123/head"
		if len(parts) != 2 {
			continue // impossible to happen
		}

		prIndex, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			log.Error("Found broken pull ref [%s] on repo %s", ref, repo.FullName())
			continue
		}

		p, err := issues_model.GetPullRequestByIndex(ctx, repo.ID, prIndex)
		if err != nil {
			if errors.Is(err, util.ErrNotExist) {
				continue // If there is no pull request for this branch, we don't try to merge it.
			}
			return nil, err
		}

		if filter(p) {
			pulls[p.ID] = p
		}
	}
	return pulls, nil
}

func handlePullRequestAutoMergeByPullID(ctx context.Context, pullID int64) {
	pr, err := issues_model.GetPullRequestByID(ctx, pullID)
	if err != nil {
		log.Error("GetPullRequestByID[%d]: %v", pullID, err)
		return
	}
	handlePullRequestAutoMerge(ctx, pr)
}

// handlePullRequestAutoMerge merge the pull request if all checks are successful
func handlePullRequestAutoMerge(ctx context.Context, pr *issues_model.PullRequest) {
	_ = pr.LoadIssue(ctx)
	if (pr.Issue != nil && pr.Issue.IsClosed) || pr.HasMerged {
		// if the PR has been closed or merged, delete the automerge record and skip
		err := pull_model.DeleteScheduledAutoMerge(ctx, pr.ID)
		if err != nil {
			log.Error("Error deleting scheduled auto merge %v: %v", pr.ID, err)
		}
		return
	}
	if !pr.IsStatusMergeable() || pr.IsWorkInProgress(ctx) {
		// quick check: if the PR can't be merged, just skip
		return
	}

	// Check if there is a scheduled pr in the db
	exists, scheduledPRM, err := pull_model.GetScheduledMergeByPullID(ctx, pr.ID)
	if err != nil {
		log.Error("%-v GetScheduledMergeByPullID: %v", pr, err)
		return
	}
	if !exists {
		return
	}

	if err = pr.LoadBaseRepo(ctx); err != nil {
		log.Error("%-v LoadBaseRepo: %v", pr, err)
		return
	}

	// Get all checks for this pr
	// We get the latest sha commit hash again to handle the case where the check of a previous push
	// did not succeed or was not finished yet.
	if err = pr.LoadHeadRepo(ctx); err != nil {
		log.Error("%-v LoadHeadRepo: %v", pr, err)
		return
	}

	switch pr.Flow {
	case issues_model.PullRequestFlowGithub:
		headBranchExist := pr.HeadRepo != nil
		if headBranchExist {
			headBranchExist, _ = git_model.IsBranchExist(ctx, pr.HeadRepo.ID, pr.HeadBranch)
		}
		if !headBranchExist {
			log.Warn("Head branch of auto merge %-v does not exist [HeadRepoID: %d, Branch: %s]", pr, pr.HeadRepoID, pr.HeadBranch)
			return
		}
	case issues_model.PullRequestFlowAGit:
		headBranchExist := git.IsReferenceExist(ctx, pr.BaseRepo, pr.GetGitHeadRefName())
		if !headBranchExist {
			log.Warn("Head branch of auto merge %-v does not exist [HeadRepoID: %d, Branch(Agit): %s]", pr, pr.HeadRepoID, pr.HeadBranch)
			return
		}
	default:
		log.Error("wrong flow type %d", pr.Flow)
		return
	}

	// Check if all checks succeeded
	pass, err := pull_service.IsPullCommitStatusPass(ctx, pr)
	if err != nil {
		log.Error("%-v IsPullCommitStatusPass: %v", pr, err)
		return
	}
	if !pass {
		log.Info("Scheduled auto merge %-v has unsuccessful status checks", pr)
		return
	}

	// Merge if all checks succeeded
	doer, err := user_model.GetUserByID(ctx, scheduledPRM.DoerID)
	if err != nil {
		log.Error("Unable to get scheduled User[%d]: %v", scheduledPRM.DoerID, err)
		return
	}

	perm, err := access_model.GetDoerRepoPermission(ctx, pr.BaseRepo, doer)
	if err != nil {
		log.Error("GetDoerRepoPermission %-v: %v", pr.BaseRepo, err)
		return
	}

	if err := pull_service.CheckPullMergeable(ctx, doer, &perm, pr, pull_service.MergeCheckTypeGeneral, scheduledPRM.MergeStyle, false); err != nil {
		if errors.Is(err, pull_service.ErrNotReadyToMerge) {
			log.Info("%-v was scheduled to automerge by an unauthorized user", pr)
			return
		}
		log.Error("%-v CheckPullMergeable: %v", pr, err)
		return
	}

	if err := pull_service.Merge(ctx, pr, doer, scheduledPRM.MergeStyle, "", scheduledPRM.Message, true); err != nil {
		log.Error("pull_service.Merge: %v", err)
		// FIXME: if merge failed, we should display some error message to the pull request page.
		// The resolution is add a new column on automerge table named `error_message` to store the error message and displayed
		// on the pull request page. But this should not be finished in a bug fix PR which will be backport to release branch.
		return
	}

	deleteBranchAfterMerge, err := pull_service.ShouldDeleteBranchAfterMerge(ctx, &scheduledPRM.DeleteBranchAfterMerge, pr.BaseRepo, pr)
	if err != nil {
		log.Error("ShouldDeleteBranchAfterMerge: %v", err)
	} else if deleteBranchAfterMerge {
		if err = repo_service.DeleteBranchAfterMerge(ctx, doer, pr.ID, nil); err != nil {
			log.Error("DeleteBranchAfterMerge: %v", err)
		}
	}
}
