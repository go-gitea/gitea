// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mergequeue

import (
	"context"
	"errors"
	"slices"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/globallock"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"
	pull_service "gitea.dev/services/pull"
)

var ErrNotQueued = util.NewNotExistErrorf("pull request is not in the merge queue")

func Enqueue(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, jump bool, expectedHeadSHA string) (*pull_model.MergeQueueEntry, bool, error) {
	if doer.ID == user_model.ActionsUserID { // a task token's permission can't be re-checked when the queue builds
		return nil, false, util.NewInvalidArgumentErrorf("Gitea Actions tokens cannot add pull requests to the merge queue")
	}
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, false, err
	}
	headSHA, err := git.GetFullCommitID(ctx, pr.BaseRepo, pr.GetGitHeadRefName())
	if err != nil {
		return nil, false, err
	} else if expectedHeadSHA != "" && expectedHeadSHA != headSHA {
		return nil, false, pull_service.ErrSHADoesNotMatch{GivenSHA: expectedHeadSHA, CurrentSHA: headSHA}
	}

	entry := &pull_model.MergeQueueEntry{
		RepoID: pr.BaseRepoID, BaseBranch: pr.BaseBranch, PullID: pr.ID,
		EnqueuerID: doer.ID, HeadSHA: headSHA, State: pull_model.MergeQueueStateQueued,
	}
	added := true
	err = globallock.LockAndDo(ctx, pull_model.MergeQueueLockKey(pr.BaseRepoID, pr.BaseBranch), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			existing, err := pull_model.GetMergeQueueEntryByPullID(ctx, pr.ID)
			if err != nil {
				return err
			}
			if existing != nil {
				if !jump {
					return pull_model.ErrAlreadyInMergeQueue
				}
				entry, added = existing, false
				return pull_model.JumpMergeQueueEntry(ctx, entry)
			}
			if jump {
				if err := pull_model.JumpMergeQueueEntry(ctx, entry); err != nil {
					return err
				}
			}
			if err := db.Insert(ctx, entry); err != nil {
				return err
			}
			if _, err := pull_model.DeleteScheduledAutoMerge(ctx, pr.ID); err != nil {
				return err
			}
			_, err = issues_model.CreatePullRequestComment(ctx, issues_model.CommentTypePRAddedToMergeQueue, pr, doer, "")
			return err
		})
	})
	if err != nil {
		return nil, false, err
	}
	if added {
		notify_service.PullRequestMergeQueueChanged(ctx, doer, pr, api.HookIssueEnqueued, "")
	}
	addToQueue(pr.BaseRepoID, pr.BaseBranch)
	return entry, added, nil
}

func Jump(ctx context.Context, pr *issues_model.PullRequest) error {
	return updateQueuedEntry(ctx, pr, pull_model.JumpMergeQueueEntry)
}

func Clear(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, branch string) error {
	return globallock.LockAndDo(ctx, pull_model.MergeQueueLockKey(repo.ID, branch), func(ctx context.Context) error {
		entries, err := loadQueue(ctx, repo, branch)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := removeEntry(ctx, repo, entry.MergeQueueEntry, entry.PullRequest, doer, issues_model.MergeQueueRemovedCleared); err != nil {
				return err
			}
		}
		return nil
	})
}

func GetEnabledRule(ctx context.Context, repo *repo_model.Repository, branch string) (*git_model.ProtectedBranch, error) {
	if exist, err := git_model.IsBranchExist(ctx, repo.ID, branch); err != nil || !exist {
		return nil, err
	}
	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, repo.ID, branch)
	if pb == nil || !pb.RequireMergeQueue {
		return nil, err
	}
	return pb, err
}

func EnqueueIfRequired(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, expectedHeadSHA string, force bool) (bool, error) {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return false, err
	}
	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil || !pull_service.MergesThroughQueue(ctx, pb, pr.BaseRepo, doer, force) {
		return false, err
	}
	if _, _, err = Enqueue(ctx, doer, pr, false, expectedHeadSHA); err != nil && !errors.Is(err, pull_model.ErrAlreadyInMergeQueue) {
		return false, err
	}
	return true, nil
}

func Dequeue(ctx context.Context, doer *user_model.User, perm access_model.Permission, pr *issues_model.PullRequest) error {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return err
	}
	if err := pr.LoadIssue(ctx); err != nil {
		return err
	}
	return updateQueuedEntry(ctx, pr, func(ctx context.Context, entry *pull_model.MergeQueueEntry) error {
		if doer.ID != entry.EnqueuerID && !pr.Issue.IsPoster(doer.ID) {
			if allowed, err := pull_service.IsUserAllowedToMerge(ctx, pr, perm, doer); err != nil {
				return err
			} else if !allowed {
				return util.NewPermissionDeniedErrorf("user is not allowed to remove the pull request from the merge queue")
			}
		}
		return removeEntry(ctx, pr.BaseRepo, entry, pr, doer, issues_model.MergeQueueRemovedByUser)
	})
}

func updateQueuedEntry(ctx context.Context, pr *issues_model.PullRequest, update func(context.Context, *pull_model.MergeQueueEntry) error) error {
	err := globallock.LockAndDo(ctx, pull_model.MergeQueueLockKey(pr.BaseRepoID, pr.BaseBranch), func(ctx context.Context) error {
		entry, err := pull_model.GetMergeQueueEntryByPullID(ctx, pr.ID)
		if err != nil {
			return err
		} else if entry == nil {
			return ErrNotQueued
		}
		return update(ctx, entry)
	})
	if err == nil {
		addToQueue(pr.BaseRepoID, pr.BaseBranch)
	}
	return err
}

type QueueEntry = pull_service.MergeQueueEntry

func loadEntries(ctx context.Context, rows []*pull_model.MergeQueueEntry, baseRepo *repo_model.Repository) ([]*QueueEntry, error) {
	if len(rows) == 0 {
		return []*QueueEntry{}, nil
	}
	pullIDs := make([]int64, len(rows))
	enqueuerIDs := make(container.Set[int64])
	for i, row := range rows {
		pullIDs[i] = row.PullID
		enqueuerIDs.Add(row.EnqueuerID)
	}
	pulls := make(map[int64]*issues_model.PullRequest, len(rows))
	if err := db.GetEngine(ctx).In("id", pullIDs).Find(&pulls); err != nil {
		return nil, err
	}
	prs := issues_model.PullRequestList(util.ValuesOfMap(pulls))
	prs.SetBaseRepo(baseRepo)
	if err := prs.LoadAttributes(ctx); err != nil {
		return nil, err
	}
	users, err := user_model.GetUsersMapByIDs(ctx, enqueuerIDs.Values())
	if err != nil {
		return nil, err
	}
	entries := make([]*QueueEntry, len(rows))
	for i, row := range rows {
		entries[i] = &QueueEntry{
			MergeQueueEntry: row, Position: i + 1,
			PullRequest: pulls[row.PullID], Enqueuer: user_model.GetPossibleUserFromMap(row.EnqueuerID, users),
		}
	}
	return entries, nil
}

func loadQueue(ctx context.Context, repo *repo_model.Repository, branch string) ([]*QueueEntry, error) {
	rows, err := pull_model.GetMergeQueueEntries(ctx, repo.ID, branch)
	if err != nil {
		return nil, err
	}
	return loadEntries(ctx, rows, repo)
}

func GetQueue(ctx context.Context, repo *repo_model.Repository, branch string) ([]*QueueEntry, error) {
	entries, err := loadQueue(ctx, repo, branch)
	return slices.DeleteFunc(entries, func(entry *QueueEntry) bool { return entry.PullRequest == nil }), err
}
