// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	org_model "code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	repo_service "code.gitea.io/gitea/services/repository"
)

func CanBlockUser(ctx context.Context, doer, blocker, blockee *user_model.User) bool {
	if blocker.ID == blockee.ID {
		return false
	}
	if doer.ID == blockee.ID {
		return false
	}

	if blockee.IsOrganization() {
		return false
	}

	if user_model.IsUserBlockedBy(ctx, blockee, blocker.ID) {
		return false
	}

	if blocker.IsOrganization() {
		org := org_model.OrgFromUser(blocker)
		if isMember, _ := org.IsOrgMember(ctx, blockee.ID); isMember {
			return false
		}
		if isAdmin, _ := org.IsOwnedBy(ctx, doer.ID); !isAdmin && !doer.IsAdmin {
			return false
		}
	} else if !doer.IsAdmin && doer.ID != blocker.ID {
		return false
	}

	return true
}

func CanUnblockUser(ctx context.Context, doer, blocker, blockee *user_model.User) bool {
	if doer.ID == blockee.ID {
		return false
	}

	if !user_model.IsUserBlockedBy(ctx, blockee, blocker.ID) {
		return false
	}

	if blocker.IsOrganization() {
		org := org_model.OrgFromUser(blocker)
		if isAdmin, _ := org.IsOwnedBy(ctx, doer.ID); !isAdmin && !doer.IsAdmin {
			return false
		}
	} else if !doer.IsAdmin && doer.ID != blocker.ID {
		return false
	}

	return true
}

func BlockUser(ctx context.Context, doer, blocker, blockee *user_model.User, note string) error {
	if blockee.IsOrganization() {
		return user_model.ErrBlockOrganization
	}

	if !CanBlockUser(ctx, doer, blocker, blockee) {
		return user_model.ErrCanNotBlock
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		// unfollow each other
		if err := user_model.UnfollowUser(ctx, blocker.ID, blockee.ID); err != nil {
			return err
		}
		if err := user_model.UnfollowUser(ctx, blockee.ID, blocker.ID); err != nil {
			return err
		}

		// unstar each other
		if err := unstarRepos(ctx, blocker, blockee); err != nil {
			return err
		}
		if err := unstarRepos(ctx, blockee, blocker); err != nil {
			return err
		}

		// unwatch each others repositories
		if err := unwatchRepos(ctx, blocker, blockee); err != nil {
			return err
		}
		if err := unwatchRepos(ctx, blockee, blocker); err != nil {
			return err
		}

		// unassign each other from issues
		if err := unassignIssues(ctx, blocker, blockee); err != nil {
			return err
		}
		if err := unassignIssues(ctx, blockee, blocker); err != nil {
			return err
		}

		// remove each other from repository collaborations
		if err := removeCollaborations(ctx, blocker, blockee); err != nil {
			return err
		}
		if err := removeCollaborations(ctx, blockee, blocker); err != nil {
			return err
		}

		// cancel each other repository transfers
		if err := cancelRepositoryTransfers(ctx, doer, blocker, blockee); err != nil {
			return err
		}
		if err := cancelRepositoryTransfers(ctx, doer, blockee, blocker); err != nil {
			return err
		}

		return db.Insert(ctx, &user_model.Blocking{
			BlockerID: blocker.ID,
			BlockeeID: blockee.ID,
			Note:      note,
		})
	})
}

func unstarRepos(ctx context.Context, starrer, repoOwner *user_model.User) error {
	opts := &repo_model.StarredReposOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 25,
		},
		StarrerID:   starrer.ID,
		RepoOwnerID: repoOwner.ID,
	}

	for {
		repos, err := repo_model.GetStarredRepos(ctx, opts)
		if err != nil {
			return err
		}

		if len(repos) == 0 {
			return nil
		}

		for _, repo := range repos {
			if err := repo_model.StarRepo(ctx, starrer, repo, false); err != nil {
				return err
			}
		}

		opts.Page++
	}
}

func unwatchRepos(ctx context.Context, watcher, repoOwner *user_model.User) error {
	opts := &repo_model.WatchedReposOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 25,
		},
		WatcherID:   watcher.ID,
		RepoOwnerID: repoOwner.ID,
	}

	for {
		repos, _, err := repo_model.GetWatchedRepos(ctx, opts)
		if err != nil {
			return err
		}

		if len(repos) == 0 {
			return nil
		}

		for _, repo := range repos {
			if err := repo_model.WatchRepo(ctx, watcher, repo, false); err != nil {
				return err
			}
		}

		opts.Page++
	}
}

func cancelRepositoryTransfers(ctx context.Context, doer, sender, recipient *user_model.User) error {
	transfers, err := repo_model.GetPendingRepositoryTransfers(ctx, &repo_model.PendingRepositoryTransferOptions{
		SenderID:    sender.ID,
		RecipientID: recipient.ID,
	})
	if err != nil {
		return err
	}

	for _, transfer := range transfers {
		if err := repo_service.CancelRepositoryTransfer(ctx, transfer, doer); err != nil {
			return err
		}
	}

	return nil
}

func unassignIssues(ctx context.Context, assignee, repoOwner *user_model.User) error {
	opts := &issues_model.AssignedIssuesOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 25,
		},
		AssigneeID:  assignee.ID,
		RepoOwnerID: repoOwner.ID,
	}

	for {
		issues, _, err := issues_model.GetAssignedIssues(ctx, opts)
		if err != nil {
			return err
		}

		if len(issues) == 0 {
			return nil
		}

		for _, issue := range issues {
			if err := issue.LoadAssignees(ctx); err != nil {
				return err
			}

			if _, _, err := issues_model.ToggleIssueAssignee(ctx, issue, assignee, assignee.ID); err != nil {
				return err
			}
		}

		opts.Page++
	}
}

func removeCollaborations(ctx context.Context, repoOwner, collaborator *user_model.User) error {
	opts := &repo_model.FindCollaborationOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 25,
		},
		CollaboratorID: collaborator.ID,
		RepoOwnerID:    repoOwner.ID,
	}

	for {
		collaborations, _, err := repo_model.GetCollaborators(ctx, opts)
		if err != nil {
			return err
		}

		if len(collaborations) == 0 {
			return nil
		}

		for _, collaboration := range collaborations {
			repo, err := repo_model.GetRepositoryByID(ctx, collaboration.Collaboration.RepoID)
			if err != nil {
				return err
			}

			if err := repo_service.DeleteCollaboration(ctx, repo, collaborator); err != nil {
				return err
			}
		}

		opts.Page++
	}
}

func UnblockUser(ctx context.Context, doer, blocker, blockee *user_model.User) error {
	if blockee.IsOrganization() {
		return user_model.ErrBlockOrganization
	}

	if !CanUnblockUser(ctx, doer, blocker, blockee) {
		return user_model.ErrCanNotUnblock
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		block, err := user_model.GetBlocking(ctx, blocker.ID, blockee.ID)
		if err != nil {
			return err
		}
		if block != nil {
			_, err = db.DeleteByID[user_model.Blocking](ctx, block.ID)
			return err
		}
		return nil
	})
}
