// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"context"
	"errors"
	"fmt"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	org_model "gitea.dev/models/organization"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"
	"gitea.dev/services/audit"
	repo_service "gitea.dev/services/repository"
)

// CreateSubgroup creates a child subgroup organization under parentOrg and recalculates repository accesses.
func CreateSubgroup(ctx context.Context, parentOrg, subOrg *org_model.Organization, creator *user_model.User) error {
	if parentOrg == nil || parentOrg.Type != user_model.UserTypeOrganization {
		var parentID int64
		if parentOrg != nil {
			parentID = parentOrg.ID
		}
		return org_model.ErrInvalidParentOrg{ParentID: parentID}
	}
	if subOrg == nil {
		return util.ErrInvalidArgument
	}
	subOrg.ParentOrgID = parentOrg.ID
	if err := org_model.CreateOrganization(ctx, subOrg, creator); err != nil {
		return err
	}
	audit.Record(ctx, audit_model.OrganizationCreate, subOrg.AsUser(), "parent_org_id", parentOrg.ID)
	return nil
}

// RecalculateSubtreeRepoAccesses refreshes access table entries for all repositories owned by rootOrgID
// and all of its descendant subgroups so ancestor owner team changes propagate deterministically.
func RecalculateSubtreeRepoAccesses(ctx context.Context, rootOrgID int64) error {
	orgIDs := []int64{rootOrgID}
	descendants, err := org_model.GetDescendantOrgs(ctx, rootOrgID, nil, true)
	if err != nil {
		return err
	}
	for _, d := range descendants {
		orgIDs = append(orgIDs, d.ID)
	}

	for _, oid := range orgIDs {
		repos, _, err := repo_model.GetUserRepositories(ctx, repo_model.SearchRepoOptions{
			OwnerID:     oid,
			Private:     true,
			ListOptions: db.ListOptionsAll,
		})
		if err != nil {
			return fmt.Errorf("RecalculateSubtreeRepoAccesses: %w", err)
		}
		for _, repo := range repos {
			if err := repo.LoadOwner(ctx); err != nil {
				return fmt.Errorf("RecalculateSubtreeRepoAccesses: %w", err)
			}
			if err := access_model.RecalculateAccesses(ctx, repo); err != nil {
				return fmt.Errorf("RecalculateSubtreeRepoAccesses: %w", err)
			}
		}
	}
	return nil
}

// recalculateDescendantUserAccess refreshes a single user's repository access across all descendant
// subgroups of parentOrgID when ancestor Owners-team membership changes, revoking watches/assignees if removed.
func recalculateDescendantUserAccess(ctx context.Context, parentOrgID int64, user *user_model.User, reconsider bool) error {
	descendants, err := org_model.GetDescendantOrgs(ctx, parentOrgID, nil, true)
	if err != nil {
		return fmt.Errorf("recalculateDescendantUserAccess: %w", err)
	}
	for _, desc := range descendants {
		repos, _, err := repo_model.GetUserRepositories(ctx, repo_model.SearchRepoOptions{
			OwnerID:     desc.ID,
			Private:     true,
			ListOptions: db.ListOptionsAll,
		})
		if err != nil {
			return fmt.Errorf("recalculateDescendantUserAccess: %w", err)
		}
		for _, repo := range repos {
			if err := access_model.RecalculateUserAccess(ctx, repo, user); err != nil && !errors.Is(err, util.ErrNotExist) {
				return fmt.Errorf("recalculateDescendantUserAccess: %w", err)
			}
			if reconsider {
				if err := repo_service.ReconsiderWatches(ctx, repo, user); err != nil && !errors.Is(err, util.ErrNotExist) {
					return fmt.Errorf("recalculateDescendantUserAccess: %w", err)
				}
				if err := repo_service.ReconsiderRepoIssuesAssignee(ctx, repo, user); err != nil && !errors.Is(err, util.ErrNotExist) {
					return fmt.Errorf("recalculateDescendantUserAccess: %w", err)
				}
			}
		}
	}
	return nil
}

// MoveSubgroup reparents subOrg under newParentOrg (or detaches subOrg to a top-level organization
// when newParentOrg is nil), serializing validation and parent updates to prevent write-skew cycles.
func MoveSubgroup(ctx context.Context, subOrg, newParentOrg *org_model.Organization, doer *user_model.User) error {
	if subOrg == nil || subOrg.Type != user_model.UserTypeOrganization {
		return util.ErrInvalidArgument
	}
	if doer == nil {
		return org_model.ErrUserNotAllowedCreateOrg{}
	}
	if newParentOrg == nil && !doer.CanCreateOrganization() {
		return org_model.ErrUserNotAllowedCreateOrg{}
	}
	if newParentOrg != nil && newParentOrg.Type != user_model.UserTypeOrganization {
		return org_model.ErrInvalidParentOrg{ParentID: newParentOrg.ID}
	}

	var committedParentID int64
	if err := org_model.WithSubgroupHierarchyLock(ctx, func(ctx context.Context) error {
		freshSub, err := org_model.GetOrgByID(ctx, subOrg.ID)
		if err != nil {
			return err
		}
		if freshSub.Type != user_model.UserTypeOrganization {
			return util.ErrInvalidArgument
		}

		if !doer.IsAdmin {
			isSubOwner, err := org_model.IsOrganizationOwner(ctx, freshSub.ID, doer.ID)
			if err != nil {
				return err
			}
			if !isSubOwner {
				return util.ErrPermissionDenied
			}
		}

		var newParentID int64
		if newParentOrg != nil {
			if !doer.IsAdmin {
				isTargetOwner, err := org_model.IsOrganizationOwner(ctx, newParentOrg.ID, doer.ID)
				if err != nil {
					return err
				}
				if !isTargetOwner {
					return util.ErrPermissionDenied
				}
			}
			if _, err := org_model.ValidateParentOrg(ctx, freshSub.ID, newParentOrg.ID, freshSub.Visibility); err != nil {
				return err
			}
			newParentID = newParentOrg.ID
		}

		committedParentID = newParentID
		if freshSub.ParentOrgID == newParentID {
			return nil
		}

		freshSub.ParentOrgID = newParentID
		if err := user_model.UpdateUserCols(ctx, freshSub.AsUser(), "parent_org_id"); err != nil {
			return err
		}
		return RecalculateSubtreeRepoAccesses(ctx, freshSub.ID)
	}); err != nil {
		return err
	}

	subOrg.ParentOrgID = committedParentID
	return nil
}
