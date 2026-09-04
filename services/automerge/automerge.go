// Copyright 2021 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package automerge

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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
	"gitea.dev/modules/timeutil"
	"gitea.dev/services/automergequeue"
	notify_service "gitea.dev/services/notify"
	pull_service "gitea.dev/services/pull"
	repo_service "gitea.dev/services/repository"
)

// Init runs the task queue to that handles auto merges
func Init(ctx context.Context) error {
	notify_service.RegisterNotifier(NewNotifier())

	automergequeue.AutoMergeQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "pr_auto_merge",
		func(items ...automergequeue.AutoMergeItem) (unhandled []automergequeue.AutoMergeItem) {
			for _, item := range items {
				handleAutoMergeItem(item)
			}
			return nil
		},
	)
	if automergequeue.AutoMergeQueue == nil {
		return errors.New("unable to create pr_auto_merge queue")
	}
	go graceful.GetManager().RunWithCancel(automergequeue.AutoMergeQueue)
	populateRecentAutoMergeItems(ctx)
	return nil
}

func populateRecentAutoMergeItems(ctx context.Context) {
	// in case Gitea's restart aborted some scheduled auto-merge pull requests, try to re-start the recent ones
	pullIDs, err := pull_model.GetScheduledMergePullIDsSince(ctx, timeutil.TimeStampNow().AddDuration(-24*time.Hour))
	if err != nil {
		log.Error("Failed to get recent scheduled auto-merge pull requests: %v", err)
		return
	}
	for _, pullID := range pullIDs {
		pull, err := issues_model.GetPullRequestByID(ctx, pullID)
		if err != nil {
			log.Error("Failed to get scheduled pull request [%d]: %v", pullID, err)
			continue
		}
		automergequeue.StartAutoMergeCheckByPullHead(ctx, pull)
	}
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
		automergequeue.StartAutoMergeCheckByPullHead(ctx, pull)
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

var errSkipAutoMerge = errors.New("skip auto merge")

func handleAutoMergeItem(item automergequeue.AutoMergeItem) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), "AutoMerge: "+string(item))
	defer finished()

	fields := strings.Split(string(item), ":")
	if len(fields) != 3 || fields[0] != "pr" {
		return
	}
	pullIDStr, headCommitID := fields[1], fields[2]
	pullID, _ := strconv.ParseInt(pullIDStr, 10, 64)
	pr, err := issues_model.GetPullRequestByID(ctx, pullID)
	if err != nil {
		log.Error("AutoMerge: GetPullRequestByID[%d]: %v", pullID, err)
		return
	}

	err = handlePullRequestAutoMerge(ctx, pr, headCommitID)
	if errors.Is(err, errSkipAutoMerge) {
		log.Debug("AutoMerge: skipping pull request [%d] auto merge: %v", pullID, err)
	} else if err != nil {
		log.Error("AutoMerge: failed to auto merge pull request [%d]: %v", pullID, err)
	} else {
		log.Info("AutoMerge: auto merge pull request [%d]", pullID)
	}
}

// handlePullRequestAutoMerge merge the pull request if all checks are successful
func handlePullRequestAutoMerge(ctx context.Context, pr *issues_model.PullRequest, expectedHeadCommitID string) error {
	_ = pr.LoadIssue(ctx)
	if (pr.Issue != nil && pr.Issue.IsClosed) || pr.HasMerged {
		// if the PR has been closed or merged, delete the automerge record and skip
		err := pull_model.DeleteScheduledAutoMerge(ctx, pr.ID)
		if err != nil {
			return errors.Join(errSkipAutoMerge, err)
		}
		return nil
	}

	if !pr.IsStatusMergeable() || pr.IsWorkInProgress(ctx) {
		// quick check: if the PR can't be merged, just skip
		return errors.Join(errSkipAutoMerge, errors.New("pull request is not mergeable or is work in progress"))
	}

	// Check if there is a scheduled pr in the db
	exists, scheduledPRM, err := pull_model.GetScheduledMergeByPullID(ctx, pr.ID)
	if err != nil {
		return fmt.Errorf("failed to get scheduled auto-merge: %w", err)
	}
	if !exists {
		return errors.Join(errSkipAutoMerge, errors.New("pull request doesn't exist"))
	}

	if err = pr.LoadBaseRepo(ctx); err != nil {
		return fmt.Errorf("failed to load base repo: %w", err)
	}
	if err = pr.LoadHeadRepo(ctx); err != nil {
		return fmt.Errorf("failed to load head repo: %w", err)
	}

	// check the sha is the same as pull request head commit id
	baseGitRepo, err := git.OpenRepository(ctx, pr.BaseRepo)
	if err != nil {
		return fmt.Errorf("failed to open base git repo: %w", err)
	}
	defer baseGitRepo.Close()

	headCommitID, err := baseGitRepo.GetRefCommitID(ctx, pr.GetGitHeadRefName())
	if err != nil {
		return fmt.Errorf("failed to get ref commit ID: %w", err)
	}
	if headCommitID != expectedHeadCommitID {
		return errors.Join(errSkipAutoMerge, errors.New("head commit ID changed"))
	}

	// Get all checks for this pr
	// We get the latest sha commit hash again to handle the case where the check of a previous push
	// did not succeed or was not finished yet.

	switch pr.Flow {
	case issues_model.PullRequestFlowGithub:
		headBranchExist := pr.HeadRepo != nil
		if headBranchExist {
			headBranchExist, _ = git_model.IsBranchExist(ctx, pr.HeadRepo.ID, pr.HeadBranch)
		}
		if !headBranchExist {
			return errors.Join(errSkipAutoMerge, errors.New("head branch does not exist"))
		}
	case issues_model.PullRequestFlowAGit:
		headBranchExist := git.IsReferenceExist(ctx, pr.BaseRepo, pr.GetGitHeadRefName())
		if !headBranchExist {
			return errors.Join(errSkipAutoMerge, errors.New("head branch (agit) does not exist"))
		}
	default:
		return errors.Join(errSkipAutoMerge, errors.New("unsupported pull request git flow type"))
	}

	// Check if all checks succeeded
	pass, err := pull_service.IsPullCommitStatusPass(ctx, pr)
	if err != nil {
		return fmt.Errorf("failed to check pull commit status: %w", err)
	}
	if !pass {
		return errors.Join(errSkipAutoMerge, errors.New("unsuccessful status checks"))
	}

	// Merge if all checks succeeded
	_, doer, err := user_model.GetPossibleUserByID(ctx, scheduledPRM.DoerID)
	if err != nil {
		return fmt.Errorf("failed to get scheduled user[%d]: %w", scheduledPRM.DoerID, err)
	}

	perm, err := access_model.GetDoerRepoPermission(ctx, pr.BaseRepo, doer)
	if err != nil {
		return fmt.Errorf("failed to get doer repo permission: %w", err)
	}

	if err := pull_service.CheckPullMergeable(ctx, doer, &perm, pr, pull_service.MergeCheckTypeGeneral, scheduledPRM.MergeStyle, false); err != nil {
		return errors.Join(errSkipAutoMerge, errors.New("pull request is not mergeable"))
	}

	if err := pull_service.Merge(pr, doer, scheduledPRM.MergeStyle, expectedHeadCommitID, scheduledPRM.Message, true); err != nil {
		// FIXME: if merge failed, we should display some error message to the pull request page, or retry later.
		// The resolution is add a new column on automerge table named `error_message` to store the error message and displayed
		// on the pull request page. But this should not be finished in a bug fix PR which will be backport to release branch.
		return fmt.Errorf("failed to merge PR:%d: %w", pr.ID, err)
	}

	// the PR has been merged, so no error should be returned after this point
	{
		deleteBranchAfterMerge, err := pull_service.ShouldDeleteBranchAfterMerge(ctx, &scheduledPRM.DeleteBranchAfterMerge, pr.BaseRepo, pr)
		if err != nil {
			log.Error("ShouldDeleteBranchAfterMerge: %v", err)
		} else if deleteBranchAfterMerge {
			if err = repo_service.DeleteBranchAfterMerge(ctx, doer, pr.ID, nil); err != nil {
				log.Error("DeleteBranchAfterMerge: %v", err)
			}
		}
	}
	return nil
}
