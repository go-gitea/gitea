// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	actions_model "code.gitea.io/gitea/models/actions"
	api "code.gitea.io/gitea/modules/structs"
)

// ToActionsTokenPermissions converts a DB ActionTokenPermissions to API ActionsTokenPermissions
func ToActionsTokenPermissions(perms *actions_model.ActionTokenPermissions) *api.ActionsTokenPermissions {
	if perms == nil {
		return nil
	}

	return &api.ActionsTokenPermissions{
		DefaultPermissions:     perms.DefaultPermissions,
		ContentsPermission:     perms.ContentsPermission,
		IssuesPermission:       perms.IssuesPermission,
		PullRequestsPermission: perms.PullRequestsPermission,
		PackagesPermission:     perms.PackagesPermission,
		MetadataPermission:     perms.MetadataPermission,
		ActionsPermission:      perms.ActionsPermission,
		OrganizationPermission: perms.OrganizationPermission,
		NotificationPermission: perms.NotificationPermission,
	}
}
