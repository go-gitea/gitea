// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	git_model "gitea.dev/models/git"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	repo_module "gitea.dev/modules/repository"
	api "gitea.dev/modules/structs"
)

// ToMergeGroupPayload also returns the queue commit, nil when it was pruned after the group was destroyed
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
	commit, _ := gitRepo.GetCommit(ctx, entry.GroupHeadSHA)
	if commit != nil {
		group.HeadCommit = ToPayloadCommit(ctx, repo, commit)
	}
	return &api.MergeGroupPayload{
		Action:     action,
		Reason:     reason,
		MergeGroup: group,
		Repo:       ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
		Sender:     ToUser(ctx, doer, nil),
	}, commit, nil
}

// ToMergeGroupPush describes a built queue commit as a push creating its branch, like GitHub's gh-readonly-queue pushes some CI builds on
func ToMergeGroupPush(doer *user_model.User, repo *repo_model.Repository, entry *pull_model.MergeQueueEntry, commit *git.Commit) (*repo_module.PushUpdateOptions, *repo_module.PushCommits) {
	commits := repo_module.GitToPushCommits([]*git.Commit{commit})
	commits.HeadCommit = commits.Commits[0]
	commits.CompareURL = repo.ComposeCompareURL(entry.BaseSHA, entry.GroupHeadSHA)
	return &repo_module.PushUpdateOptions{
		PusherID:     doer.ID,
		PusherName:   doer.Name,
		RepoUserName: repo.OwnerName,
		RepoName:     repo.Name,
		RefFullName:  git.RefName(entry.GroupRef),
		OldCommitID:  git.ObjectFormatFromName(repo.ObjectFormatName).EmptyObjectID().String(),
		NewCommitID:  entry.GroupHeadSHA,
	}, commits
}

func ToBranchProtectionMergeQueue(cfg git_model.MergeQueueConfig) *api.BranchProtectionMergeQueue {
	cfg = cfg.WithDefaults()
	return &api.BranchProtectionMergeQueue{
		MergeMethod:                  string(cfg.MergeMethod),
		MaxEntriesToBuild:            cfg.MaxEntriesToBuild,
		MinEntriesToMerge:            cfg.MinEntriesToMerge,
		MaxEntriesToMerge:            cfg.MaxEntriesToMerge,
		MinEntriesToMergeWaitMinutes: cfg.MinEntriesToMergeWaitMinutes,
		GroupingStrategy:             string(cfg.GroupingStrategy),
		CheckResponseTimeoutMinutes:  cfg.CheckResponseTimeoutMinutes,
	}
}

func ToMergeQueueEntry(ctx context.Context, entry *pull_model.MergeQueueEntry, position int, prIndex int64, enqueuer, viewer *user_model.User) *api.MergeQueueEntry {
	return &api.MergeQueueEntry{
		Position:          position,
		State:             string(entry.State),
		PullRequestIndex:  prIndex,
		BaseBranch:        entry.BaseBranch,
		HeadSHA:           entry.HeadSHA,
		MergeGroupHeadSHA: entry.GroupHeadSHA,
		MergeGroupHeadRef: entry.GroupRef,
		Enqueuer:          ToUser(ctx, enqueuer, viewer),
		EnqueuedAt:        entry.CreatedUnix.AsTime(),
	}
}
