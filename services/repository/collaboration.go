// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"

	"xorm.io/builder"
)

func AddOrUpdateCollaborator(ctx context.Context, repo *repo_model.Repository, u *user_model.User, mode perm.AccessMode) error {
	// only allow valid access modes, read, write and admin
	if mode < perm.AccessModeRead || mode > perm.AccessModeAdmin {
		return perm.ErrInvalidAccessMode
	}

	if err := repo.LoadOwner(ctx); err != nil {
		return err
	}

	if user_model.IsUserBlockedBy(ctx, u, repo.OwnerID) || user_model.IsUserBlockedBy(ctx, repo.Owner, u.ID) {
		return user_model.ErrBlockedUser
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		collaboration, has, err := db.Get[repo_model.Collaboration](ctx, builder.Eq{
			"repo_id": repo.ID,
			"user_id": u.ID,
		})
		if err != nil {
			return err
		} else if has {
			if collaboration.Mode == mode {
				return nil
			}
			if _, err = db.GetEngine(ctx).
				Where("repo_id=?", repo.ID).
				And("user_id=?", u.ID).
				Cols("mode").
				Update(&repo_model.Collaboration{
					Mode: mode,
				}); err != nil {
				return err
			}
		} else if err = db.Insert(ctx, &repo_model.Collaboration{
			RepoID: repo.ID,
			UserID: u.ID,
			Mode:   mode,
		}); err != nil {
			return err
		}

		return access_model.RecalculateUserAccess(ctx, repo, u.ID)
	})
}

// DeleteCollaboration removes collaboration relation between the user and repository.
func DeleteCollaboration(ctx context.Context, repo *repo_model.Repository, collaborator *user_model.User) (err error) {
	collaboration := &repo_model.Collaboration{
		RepoID: repo.ID,
		UserID: collaborator.ID,
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if has, err := db.GetEngine(ctx).Delete(collaboration); err != nil {
		return err
	} else if has == 0 {
		return committer.Commit()
	}

	if err := repo.LoadOwner(ctx); err != nil {
		return err
	}

	if err = access_model.RecalculateAccesses(ctx, repo); err != nil {
		return err
	}

	if err = repo_model.WatchRepo(ctx, collaborator, repo, false); err != nil {
		return err
	}

	if err = ReconsiderWatches(ctx, repo, collaborator); err != nil {
		return err
	}

	// Unassign a user from any issue (s)he has been assigned to in the repository
	if err := ReconsiderRepoIssuesAssignee(ctx, repo, collaborator); err != nil {
		return err
	}

	return committer.Commit()
}

func ReconsiderRepoIssuesAssignee(ctx context.Context, repo *repo_model.Repository, user *user_model.User) error {
	if canAssigned, err := access_model.CanBeAssigned(ctx, user, repo, true); err != nil || canAssigned {
		return err
	}

	if _, err := db.GetEngine(ctx).Where(builder.Eq{"assignee_id": user.ID}).
		In("issue_id", builder.Select("id").From("issue").Where(builder.Eq{"repo_id": repo.ID})).
		Delete(&issues_model.IssueAssignees{}); err != nil {
		return fmt.Errorf("Could not delete assignee[%d] %w", user.ID, err)
	}
	return nil
}

func ReconsiderWatches(ctx context.Context, repo *repo_model.Repository, user *user_model.User) error {
	if has, err := access_model.HasAnyUnitAccess(ctx, user.ID, repo); err != nil || has {
		return err
	}
	if err := repo_model.WatchRepo(ctx, user, repo, false); err != nil {
		return err
	}

	// Remove all IssueWatches a user has subscribed to in the repository
	return issues_model.RemoveIssueWatchersByRepoID(ctx, user.ID, repo.ID)
}
