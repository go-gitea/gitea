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
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"
	pull_service "gitea.dev/services/pull"
)

var ErrNotQueued = util.NewNotExistErrorf("pull request is not in the merge queue")

// Enqueue relies on the caller to check the branch rule and merge permission.
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
		RepoID: pr.BaseRepoID, BaseBranch: pr.BaseBranch, PullID: pr.ID,
		EnqueuerID: doer.ID, Jump: jump, HeadSHA: headSHA,
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
	notify_service.PullRequestMergeQueueChanged(ctx, doer, pr, "")
	addToQueue(pr.BaseRepoID, pr.BaseBranch)
	return entry, nil
}

func EnqueueIfRequired(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, expectedHeadSHA string, force bool) (bool, error) {
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
	if err == nil {
		addToQueue(pr.BaseRepoID, pr.BaseBranch)
	}
	return err
}

type QueueEntry struct {
	*pull_model.MergeQueueEntry
	Position    int
	PullRequest *issues_model.PullRequest
	Enqueuer    *user_model.User
}

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
	if _, err := prs.LoadIssues(ctx); err != nil {
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

func GetQueue(ctx context.Context, repo *repo_model.Repository, branch string) ([]*QueueEntry, error) {
	rows, err := pull_model.GetMergeQueueEntries(ctx, repo.ID, branch)
	if err != nil {
		return nil, err
	}
	entries, err := loadEntries(ctx, rows, repo)
	return slices.DeleteFunc(entries, func(entry *QueueEntry) bool { return entry.PullRequest == nil }), err
}

func GetEntry(ctx context.Context, pr *issues_model.PullRequest) (*pull_model.MergeQueueEntry, int, error) {
	entry, err := pull_model.GetMergeQueueEntryByPullID(ctx, pr.ID)
	if err != nil || entry == nil {
		return nil, 0, err
	}
	position, err := pull_model.GetMergeQueuePosition(ctx, entry)
	return entry, position, err
}
