// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	repo_module "gitea.dev/modules/repository"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

var mergeQueueRemovalReasons = map[issues_model.MergeQueueRemovalReason]string{
	issues_model.MergeQueueRemovedByUser:        "MANUAL",
	issues_model.MergeQueueRemovedClosed:        "MANUAL",
	issues_model.MergeQueueRemovedMerged:        "MERGE",
	issues_model.MergeQueueRemovedAlreadyMerged: "ALREADY_MERGED",
	issues_model.MergeQueueRemovedChecksFailed:  "CI_FAILURE",
	issues_model.MergeQueueRemovedChecksTimeout: "CI_TIMEOUT",
	issues_model.MergeQueueRemovedConflict:      "MERGE_CONFLICT",
	issues_model.MergeQueueRemovedNotMergeable:  "BRANCH_PROTECTIONS",
	issues_model.MergeQueueRemovedBaseDeleted:   "QUEUE_CLEARED",
	issues_model.MergeQueueRemovedCleared:       "QUEUE_CLEARED",
}

func ToMergeQueueRemovalReason(reason issues_model.MergeQueueRemovalReason) string {
	if reason == "" {
		return ""
	}
	return util.IfZero(mergeQueueRemovalReasons[reason], "UNKNOWN_REMOVAL_REASON")
}

func ToMergeGroupPayload(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, entry *pull_model.MergeQueueEntry, action api.HookMergeGroupAction, reason string) (*api.MergeGroupPayload, *git.Commit, error) {
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return nil, nil, err
	}
	defer closer.Close()

	group := &api.MergeGroup{
		HeadSHA: entry.GroupHeadSHA,
		HeadRef: entry.GroupRef,
		BaseSHA: entry.BaseSHA,
		BaseRef: git.RefNameFromBranch(entry.BaseBranch).String(),
	}
	commit, err := gitRepo.GetCommit(ctx, entry.GroupHeadSHA)
	if err == nil {
		group.HeadCommit = ToPayloadCommit(ctx, repo, commit)
	} else if !git.IsErrNotExist(err) { // a destroyed group's commit may have been pruned
		return nil, nil, err
	}
	return &api.MergeGroupPayload{
		Action:     action,
		Reason:     reason,
		MergeGroup: group,
		Repo:       ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
		Sender:     ToUser(ctx, doer, nil),
	}, commit, nil
}

func ToMergeGroupPush(repo *repo_model.Repository, entry *pull_model.MergeQueueEntry, commit *git.Commit) (*repo_module.PushUpdateOptions, *repo_module.PushCommits) {
	commits := repo_module.GitToPushCommits([]*git.Commit{commit})
	commits.HeadCommit = commits.Commits[0]
	commits.CompareURL = repo.ComposeCompareURL(entry.BaseSHA, entry.GroupHeadSHA)
	return &repo_module.PushUpdateOptions{
		RefFullName: git.RefName(entry.GroupRef),
		OldCommitID: git.ObjectFormatFromName(repo.ObjectFormatName).EmptyObjectID().String(),
		NewCommitID: entry.GroupHeadSHA,
	}, commits
}

func ToMergeQueueEntry(ctx context.Context, entry *pull_model.MergeQueueEntry, position int, prIndex int64, enqueuer *user_model.User) *api.MergeQueueEntry {
	return &api.MergeQueueEntry{
		Position:          position,
		State:             api.MergeQueueEntryState(entry.State),
		PullRequestIndex:  prIndex,
		BaseBranch:        entry.BaseBranch,
		HeadSHA:           entry.HeadSHA,
		MergeGroupHeadSHA: entry.GroupHeadSHA,
		MergeGroupHeadRef: entry.GroupRef,
		Jump:              entry.JumpUnix > 0,
		Enqueuer:          ToUser(ctx, enqueuer, nil),
		EnqueuedAt:        entry.CreatedUnix.AsTime(),
	}
}
