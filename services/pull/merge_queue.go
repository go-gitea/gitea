// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"
	"strings"

	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/log"
	repo_module "gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
)

// MergesThroughQueue reports whether a merge by the doer enters the merge queue of the rule, only a forced merge that bypasses the protection merges directly
func MergesThroughQueue(ctx context.Context, pb *git_model.ProtectedBranch, repo *repo_model.Repository, doer *user_model.User, force bool) bool {
	return pb != nil && pb.RequireMergeQueue && !(force && git_model.CanBypassBranchProtection(ctx, pb, doer, access_model.IsUserRepoAdmin(ctx, repo, doer)))
}

// CreateMergeQueueCommit merges the pull request onto baseCommitID and stores the result at targetRef in the base repo.
// The ref is written without running hooks, like GitHub's queue branches it is not a user branch.
func CreateMergeQueueCommit(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User, baseGitRepo *git.Repository, mergeStyle repo_model.MergeStyle, baseCommitID, expectedHeadCommitID, targetRef string) (string, error) {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return "", err
	}
	message, body, err := GetDefaultMergeMessage(ctx, baseGitRepo, pr, mergeStyle)
	if err != nil {
		return "", err
	}
	if mergeStyle == repo_model.MergeStyleSquash {
		squashCommitMessages, err := GetSquashMergeCommitMessages(ctx, pr)
		if err != nil {
			return "", err
		}
		body = git.CommitMessageMerge(squashCommitMessages, body)
	}
	if body != "" {
		message += "\n\n" + body
	}

	mergeCtx, cancel, err := createTemporaryRepoForMerge(ctx, pr, doer, expectedHeadCommitID, baseCommitID)
	if err != nil {
		return "", err
	}
	defer cancel()

	if err := doMergeStyle(mergeCtx, mergeStyle, message); err != nil {
		return "", ErrMergeQueueMerge{err}
	}
	commitID, err := git.GetFullCommitID(ctx, mergeCtx.tmpRepo, tmpRepoBaseBranch)
	if err != nil {
		return "", fmt.Errorf("CreateMergeQueueCommit: %w", err)
	}
	if setting.LFS.StartServer {
		if err := LFSPush(ctx, mergeCtx.tmpBasePath, mergeCtx.tmpRepo, commitID, baseCommitID, pr); err != nil {
			return "", err
		}
	}

	if err := git.FetchRemoteCommitUpdateRef(ctx, pr.BaseRepo, mergeCtx.tmpRepo, git.BranchPrefix+tmpRepoBaseBranch, targetRef); err != nil {
		return "", fmt.Errorf("CreateMergeQueueCommit: fetch into %s: %w", targetRef, err)
	}
	return commitID, nil
}

type ErrMergeQueueMerge struct{ error }

func (err ErrMergeQueueMerge) Unwrap() error { return err.error }

// MergeQueueRequiredContexts adapts required contexts to queue commits: Actions name their statuses after the event,
// so a required "CI / test (pull_request)" is satisfied by "CI / test (merge_group)" like GitHub's event-less check names
func MergeQueueRequiredContexts(required []string) []string {
	adapted := make([]string, len(required))
	for i, name := range required {
		if prefix, event, _ := strings.CutLast(name, " "); event == "(pull_request)" || event == "(pull_request_target)" {
			name = prefix + " (merge_group)"
		}
		adapted[i] = name
	}
	return adapted
}

// MergeQueueItem is a pull request merged by the queue, as the user who enqueued it, with the queue commit that added it
type MergeQueueItem struct {
	PR       *issues_model.PullRequest
	Doer     *user_model.User
	HeadSHA  string
	CommitID string
}

// MergeQueuePullRequests fast-forwards the base branch to the last queue commit and marks the pull requests merged.
// The push runs the hooks so the usual push events are sent.
func MergeQueuePullRequests(ctx context.Context, repo *repo_model.Repository, baseBranch string, items []MergeQueueItem) error {
	last := items[len(items)-1]
	// like Merge, keep the pull request checker from marking them merged before this does
	for _, item := range items {
		release, err := globallock.Lock(ctx, getPullWorkingLockKey(item.PR.ID))
		if err != nil {
			return err
		}
		defer release()
	}
	for _, item := range items {
		pr, err := issues_model.GetPullRequestByID(ctx, item.PR.ID)
		if err != nil {
			return err
		}
		if err := pr.LoadIssue(ctx); err != nil {
			return err
		}
		if pr.HasMerged || pr.Issue.IsClosed {
			return fmt.Errorf("MergeQueuePullRequests: %-v was merged or closed", pr)
		}
		if pr.BaseRepoID != repo.ID || pr.BaseBranch != baseBranch {
			return fmt.Errorf("MergeQueuePullRequests: %-v no longer targets %s", pr, baseBranch)
		}
		headSHA, err := git.GetFullCommitID(ctx, repo, pr.GetGitHeadRefName())
		if err != nil {
			return err
		} else if headSHA != item.HeadSHA {
			return ErrSHADoesNotMatch{GivenSHA: item.HeadSHA, CurrentSHA: headSHA}
		}
	}
	var err error
	for _, item := range items {
		if err = recordMergeIntent(ctx, item.PR, item.Doer, item.CommitID, false); err != nil {
			break
		}
	}
	if err == nil {
		if err = git.PushManaged(ctx, repo, repo.CodeStorageRepo(), git.PushOptions{
			Branch: last.CommitID + ":" + git.BranchPrefix + baseBranch,
			Env:    repo_module.FullPushingEnvironment(last.Doer, last.Doer, repo, repo.Name, last.PR.ID, last.PR.Index),
		}); err != nil {
			err = fmt.Errorf("push to %s: %w", baseBranch, err)
		}
	}
	if err != nil {
		for _, item := range items {
			// like Merge, keep the intent unless the commit verifiably didn't reach the base branch, so the checker can still mark it merged
			if merged, checkErr := hasPullRequestCommitBeenMerged(ctx, item.PR); checkErr != nil || merged || item.PR.MergedCommitID == "" {
				continue
			}
			if clearErr := clearMergeIntent(ctx, item.PR); clearErr != nil {
				log.Error("Unable to clear merge intent of %-v: %v", item.PR, clearErr)
			}
		}
		return fmt.Errorf("MergeQueuePullRequests: %w", err)
	}

	for _, item := range items {
		item.PR.HasMerged, err = MarkAsMerged(ctx, item.PR, item.PR.MergedCommitID, item.PR.MergedUnix, item.Doer, item.PR.Status)
		if err != nil {
			log.Error("MarkAsMerged %-v: %v", item.PR, err)
		}
	}
	addTestPullRequestTaskAfterWebOperation(last.PR, last.Doer)
	return nil
}
