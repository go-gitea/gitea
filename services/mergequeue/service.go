// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mergequeue

import (
	"context"
	"errors"
	"maps"
	"slices"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	pull_model "gitea.dev/models/pull"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"
	pull_service "gitea.dev/services/pull"
)

var ErrNotQueued = util.NewNotExistErrorf("pull request is not in the merge queue")

// Enqueue adds the pull request to the merge queue, the caller checks that its base branch requires one and that the doer may merge it.
// A non-empty expectedHeadSHA must still be the head, like for a direct merge.
func Enqueue(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, jump bool, expectedHeadSHA string) (*pull_model.MergeQueueEntry, error) {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, err
	}
	headSHA, err := git.GetFullCommitID(ctx, pr.BaseRepo, pr.GetGitHeadRefName())
	if err != nil {
		return nil, err
	} else if expectedHeadSHA != "" && expectedHeadSHA != headSHA {
		return nil, pull_service.ErrSHADoesNotMatch{GivenSHA: expectedHeadSHA, CurrentSHA: headSHA}
	}

	entry := &pull_model.MergeQueueEntry{
		RepoID:     pr.BaseRepoID,
		BaseBranch: pr.BaseBranch,
		PullID:     pr.ID,
		EnqueuerID: doer.ID,
		Jump:       jump,
		HeadSHA:    headSHA,
	}
	err = globallock.LockAndDo(ctx, lockKey(pr.BaseRepoID, pr.BaseBranch), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			if err := pull_model.AddToMergeQueue(ctx, entry); err != nil {
				return err
			}
			return issues_model.CreateMergeQueueComment(ctx, pr, doer, "")
		})
	})
	if err != nil {
		return nil, err
	}
	notify_service.PullRequestMergeQueueChanged(ctx, doer, pr, true, "")
	addToQueue(pr.BaseRepoID, pr.BaseBranch)
	return entry, nil
}

// EnqueueIfRequired adds the pull request to the merge queue when its merge goes through one, an already queued one counts as added
func EnqueueIfRequired(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, expectedHeadSHA string, force bool) (queued bool, err error) {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return false, err
	}
	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil || !pull_service.MergesThroughQueue(ctx, pb, pr.BaseRepo, doer, force) {
		return false, err
	}
	if _, err = Enqueue(ctx, doer, pr, false, expectedHeadSHA); err != nil && !errors.Is(err, pull_model.ErrAlreadyInMergeQueue) {
		return false, err
	}
	return true, nil
}

// Dequeue removes the pull request from the merge queue, the entries after it are rebuilt without it.
// The enqueuer and users allowed to merge may do so.
func Dequeue(ctx context.Context, doer *user_model.User, perm access_model.Permission, pr *issues_model.PullRequest) error {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return err
	}
	err := globallock.LockAndDo(ctx, lockKey(pr.BaseRepoID, pr.BaseBranch), func(ctx context.Context) error {
		entry, err := pull_model.GetMergeQueueEntryByPullID(ctx, pr.ID)
		if err != nil {
			return err
		} else if entry == nil {
			return ErrNotQueued
		}
		if doer.ID != entry.EnqueuerID {
			if allowed, err := pull_service.IsUserAllowedToMerge(ctx, pr, perm, doer); err != nil {
				return err
			} else if !allowed {
				return util.NewPermissionDeniedErrorf("user is not allowed to remove the pull request from the merge queue")
			}
		}
		return removeEntry(ctx, pr.BaseRepo, entry, pr, doer, issues_model.MergeQueueRemovedByUser)
	})
	if err != nil {
		return err
	}
	addToQueue(pr.BaseRepoID, pr.BaseBranch)
	return nil
}

type QueueEntry struct {
	*pull_model.MergeQueueEntry
	Position    int
	PullRequest *issues_model.PullRequest
	Enqueuer    *user_model.User
}

// loadPulls loads the pull requests of the entries with their issues, deleted ones are missing
func loadPulls(ctx context.Context, entries []*pull_model.MergeQueueEntry) (map[int64]*issues_model.PullRequest, error) {
	prs := make(map[int64]*issues_model.PullRequest, len(entries))
	if err := db.GetEngine(ctx).In("id", container.FilterSlice(entries, func(e *pull_model.MergeQueueEntry) (int64, bool) { return e.PullID, true })).Find(&prs); err != nil {
		return nil, err
	}
	_, err := issues_model.PullRequestList(slices.Collect(maps.Values(prs))).LoadIssues(ctx)
	return prs, err
}

func loadEnqueuers(ctx context.Context, entries []*pull_model.MergeQueueEntry) (map[int64]*user_model.User, error) {
	return user_model.GetUsersMapByIDs(ctx, container.FilterSlice(entries, func(e *pull_model.MergeQueueEntry) (int64, bool) { return e.EnqueuerID, true }))
}

func GetQueue(ctx context.Context, repoID int64, branch string) ([]*QueueEntry, error) {
	entries, err := pull_model.GetMergeQueueEntries(ctx, repoID, branch)
	if err != nil {
		return nil, err
	}
	prs, err := loadPulls(ctx, entries)
	if err != nil {
		return nil, err
	}
	users, err := loadEnqueuers(ctx, entries)
	if err != nil {
		return nil, err
	}
	result := make([]*QueueEntry, 0, len(entries))
	for i, e := range entries {
		if pr := prs[e.PullID]; pr != nil {
			result = append(result, &QueueEntry{MergeQueueEntry: e, Position: i + 1, PullRequest: pr, Enqueuer: user_model.GetPossibleUserFromMap(e.EnqueuerID, users)})
		}
	}
	return result, nil
}

// GetEntry returns the queue entry of the pull request and its 1-based position, nil when it is not queued
func GetEntry(ctx context.Context, pr *issues_model.PullRequest) (*pull_model.MergeQueueEntry, int, error) {
	entry, err := pull_model.GetMergeQueueEntryByPullID(ctx, pr.ID)
	if err != nil || entry == nil {
		return nil, 0, err
	}
	position, err := pull_model.GetMergeQueuePosition(ctx, entry)
	return entry, position, err
}
