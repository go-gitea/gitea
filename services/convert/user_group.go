// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"code.gitea.io/gitea/models/usergroup"
	api "code.gitea.io/gitea/modules/structs"
)

// ToUserGroup converts a UserGroup model to API format.
func ToUserGroup(group *usergroup.UserGroup) *api.UserGroup {
	return &api.UserGroup{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		ParentID:    group.ParentID,
	}
}
