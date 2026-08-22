// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/services/audit"

	"xorm.io/builder"
)

func AddOrUpdateCollaborator(ctx context.Context, repo *repo_model.Repository, u *user_model.User, mode perm.AccessMode) error {
	// Only allow valid access modes, read, write and admin
	// Keep in mind: do not allow "owner" here: because "admin" user can update collaborators but not make dangerous operations.
	// If the "admin" user updates a user to "owner", then it means that the admin user can use owner permission, which is not expected.
	if mode < perm.AccessModeRead || mode > perm.AccessModeAdmin {
		return perm.ErrInvalidAccessMode
	}

	if err := repo.LoadOwner(ctx); err != nil {
		return err
	}

	if user_model.IsUserBlockedBy(ctx, u, repo.OwnerID) || user_model.IsUserBlockedBy(ctx, repo.Owner, u.ID) {
		return user_model.ErrBlockedUser
	}

	added, updated := false, false
	if err := db.WithTx(ctx, func(ctx context.Context) error {
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
			updated = true
		} else {
			if err = db.Insert(ctx, &repo_model.Collaboration{
				RepoID: repo.ID,
				UserID: u.ID,
				Mode:   mode,
			}); err != nil {
				return err
			}
			added = true
		}

		return access_model.RecalculateUserAccess(ctx, repo, u.ID)
	}); err != nil {
		return err
	}

	switch {
	case added:
		audit.Record(ctx, audit_model.RepositoryCollaboratorAdd, repo, "collaborator", u.Name, "access_mode", mode.ToString())
	case updated:
		audit.Record(ctx, audit_model.RepositoryCollaboratorAccess, repo, "collaborator", u.Name, "access_mode", mode.ToString())
	}

	return nil
}

// DeleteCollaboration removes collaboration relation between the user and repository.
func DeleteCollaboration(ctx context.Context, repo *repo_model.Repository, collaborator *user_model.User) error {
	return deleteCollaboration(ctx, repo, collaborator, &repo_model.Collaboration{RepoID: repo.ID, UserID: collaborator.ID})
}

func deleteCollaborationByMode(ctx context.Context, repo *repo_model.Repository, collaborator *user_model.User, mode perm.AccessMode) error {
	return deleteCollaboration(ctx, repo, collaborator, &repo_model.Collaboration{
		RepoID: repo.ID, UserID: collaborator.ID, Mode: mode,
	})
}

func deleteCollaboration(ctx context.Context, repo *repo_model.Repository, collaborator *user_model.User, collaboration *repo_model.Collaboration) (err error) {
	deleted := false
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if n, err := db.GetEngine(ctx).Delete(collaboration); err != nil {
			return err
		} else if n == 0 {
			return nil
		}
		deleted = true

		if err := repo.LoadOwner(ctx); err != nil {
			return err
		}
		if err = access_model.RecalculateAccesses(ctx, repo); err != nil {
			return err
		}

		if err = ReconsiderWatches(ctx, repo, collaborator); err != nil {
			return err
		}

		// Unassign a user from any issue (s)he has been assigned to in the repository
		return ReconsiderRepoIssuesAssignee(ctx, repo, collaborator)
	}); err != nil {
		return err
	}

	if deleted {
		audit.Record(ctx, audit_model.RepositoryCollaboratorRemove, repo, "collaborator", collaborator.Name)
	}

	return nil
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
