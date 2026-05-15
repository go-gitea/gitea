// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package usergroup

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/usergroup"

	"xorm.io/builder"
)

// DeleteUserGroup deletes a user group if it has no child groups.
func DeleteUserGroup(ctx context.Context, groupID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		exists, err := db.Exist[usergroup.UserGroup](ctx, builder.Eq{"id": groupID})
		if err != nil {
			return err
		} else if !exists {
			return usergroup.ErrUserGroupNotExist{GroupID: groupID}
		}

		hasChildren, err := db.GetEngine(ctx).Where("parent_id=?", groupID).Exist(new(usergroup.UserGroup))
		if err != nil {
			return err
		} else if hasChildren {
			return usergroup.ErrUserGroupHasChildren{GroupID: groupID}
		}

		return db.DeleteBeans(ctx,
			&usergroup.UserGroup{ID: groupID},
			&usergroup.UserGroupMember{GroupID: groupID},
			&organization.TeamUserGroup{GroupID: groupID},
		)
	})
}
