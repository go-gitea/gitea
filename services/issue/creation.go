// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"gitea.dev/models/issues"
	"gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/util"
)

func CheckCreationRestriction(perm access.Permission, unitType unit.Type, restriction string) error {
	switch restriction {
	case repo_model.CreationRestrictionMember:
		if !perm.CanRead(unitType) {
			return issues.Errors().MustRepoUnitReader
		}
		if !perm.IsMemberOrCollaborator() {
			return issues.Errors().MustMemberOrCollaborator
		}
		return nil
	case repo_model.CreationRestrictionWriter:
		return util.Iif(perm.CanWrite(unitType), nil, issues.Errors().MustRepoUnitWriter)
	default:
		return util.Iif(perm.CanRead(unitType), nil, issues.Errors().MustRepoUnitReader)
	}
}
