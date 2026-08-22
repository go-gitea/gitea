// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"

	"xorm.io/builder"
)

func validateCollaboration(ctx context.Context, repo *repo_model.Repository, user *user_model.User, mode perm.AccessMode) error {
	// Collaborators cannot receive owner access.
	if mode < perm.AccessModeRead || mode > perm.AccessModeAdmin {
		return perm.ErrInvalidAccessMode
	}

	if err := repo.LoadOwner(ctx); err != nil {
		return err
	}

	if user_model.IsUserBlockedBy(ctx, user, repo.OwnerID) || user_model.IsUserBlockedBy(ctx, repo.Owner, user.ID) {
		return user_model.ErrBlockedUser
	}
	return nil
}

func insertCollaborator(ctx context.Context, repo *repo_model.Repository, user *user_model.User, mode perm.AccessMode) (int64, error) {
	if err := validateCollaboration(ctx, repo, user, mode); err != nil {
		return 0, err
	}
	return db.WithTx2(ctx, func(ctx context.Context) (int64, error) {
		collaboration := &repo_model.Collaboration{RepoID: repo.ID, UserID: user.ID, Mode: mode}
		if err := db.Insert(ctx, collaboration); err != nil {
			return 0, err
		}
		return collaboration.ID, access_model.RecalculateUserAccess(ctx, repo, user.ID)
	})
}

func AddOrUpdateCollaborator(ctx context.Context, repo *repo_model.Repository, u *user_model.User, mode perm.AccessMode) error {
	if err := validateCollaboration(ctx, repo, u, mode); err != nil {
		return err
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
func DeleteCollaboration(ctx context.Context, repo *repo_model.Repository, collaborator *user_model.User) error {
	return deleteCollaboration(ctx, repo, collaborator, builder.Eq{"repo_id": repo.ID, "user_id": collaborator.ID})
}

func deleteCollaborationByIDAndMode(ctx context.Context, repo *repo_model.Repository, collaborator *user_model.User, id int64, mode perm.AccessMode) error {
	return deleteCollaboration(ctx, repo, collaborator, builder.Eq{
		"id": id, "repo_id": repo.ID, "user_id": collaborator.ID, "mode": mode,
	})
}

func deleteCollaboration(ctx context.Context, repo *repo_model.Repository, collaborator *user_model.User, condition builder.Cond) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if deleted, err := db.GetEngine(ctx).Where(condition).Delete(new(repo_model.Collaboration)); err != nil {
			return err
		} else if deleted == 0 {
			return nil
		}

		if err := repo.LoadOwner(ctx); err != nil {
			return err
		}
		if err := access_model.RecalculateAccesses(ctx, repo); err != nil {
			return err
		}

		if err := ReconsiderWatches(ctx, repo, collaborator); err != nil {
			return err
		}

		// Unassign a user from any issue (s)he has been assigned to in the repository
		return ReconsiderRepoIssuesAssignee(ctx, repo, collaborator)
	})
}

func ReconsiderRepoIssuesAssignee(ctx context.Context, repo *repo_model.Repository, user *user_model.User) error {
	if canAssigned, err := access_model.CanBeAssigned(ctx, user, repo); err != nil || canAssigned {
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
	permission, err := access_model.GetIndividualUserRepoPermission(ctx, repo, user)
	if err != nil || permission.HasAnyUnitAccessOrPublicAccess() {
		return err
	}
	if err := repo_model.WatchRepoAuto(ctx, user, repo, false); err != nil {
		return err
	}

	// Remove all stopwatches a user has running in the repository
	if err := issues_model.RemoveStopwatchesByRepoID(ctx, user.ID, repo.ID); err != nil {
		return err
	}

	// Remove all IssueWatches a user has subscribed to in the repository
	return issues_model.RemoveIssueWatchersByRepoID(ctx, user.ID, repo.ID)
}
