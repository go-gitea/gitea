// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package access

import (
	"context"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
)

// CreationCheckResult describes the outcome of CheckCreationRestriction.
type CreationCheckResult int

const (
	// CreationAllowed means the doer is allowed to create the issue/pull request
	CreationAllowed CreationCheckResult = iota
	// CreationDeniedNotCollaborator means the repo unit requires at least CreationRestrictionMember
	// and the doer is not an effective collaborator (owner, team member, or explicit collaborator)
	CreationDeniedNotCollaborator
	// CreationDeniedNeedsWriteAccess means the repo unit requires CreationRestrictionWriter
	// and the doer, while an effective collaborator, does not have write access to the unit
	CreationDeniedNeedsWriteAccess
)

// IsEffectiveCollaborator reports whether the user has access that was granted explicitly
// (repo owner, organization team membership, or an explicit Collaboration row), as opposed to
// access that is only derived from the repository being public/visible to everyone.
func IsEffectiveCollaborator(ctx context.Context, repo *repo_model.Repository, userID int64) (bool, error) {
	return repo_model.HasAccessToRepoCodeUnit(ctx, repo, userID)
}

// CheckCreationRestriction enforces the given CreationRestriction policy for the unit (issues or
// pull requests) against the doer. It never performs a visibility/read check itself: callers are
// expected to have already verified the doer can see the repository and unit.
func CheckCreationRestriction(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, unitType unit.Type, restriction repo_model.CreationRestriction) (CreationCheckResult, error) {
	if restriction == repo_model.CreationRestrictionReader {
		return CreationAllowed, nil
	}
	if doer == nil {
		return CreationDeniedNotCollaborator, nil
	}
	if doer.IsAdmin {
		return CreationAllowed, nil
	}

	isCollaborator, err := IsEffectiveCollaborator(ctx, repo, doer.ID)
	if err != nil {
		return CreationAllowed, err
	}
	if !isCollaborator {
		return CreationDeniedNotCollaborator, nil
	}
	if restriction == repo_model.CreationRestrictionMember {
		return CreationAllowed, nil
	}

	// CreationRestrictionWriter: the doer must have write access to the unit
	perm, err := GetDoerRepoPermission(ctx, repo, doer)
	if err != nil {
		return CreationAllowed, err
	}
	if !perm.CanWrite(unitType) {
		return CreationDeniedNeedsWriteAccess, nil
	}
	return CreationAllowed, nil
}
