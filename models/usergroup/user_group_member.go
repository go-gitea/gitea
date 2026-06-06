// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package usergroup

import (
	"context"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"

	"xorm.io/builder"
)

// Member represents a user membership in a user group.
type Member struct {
	GroupID int64 `xorm:"UNIQUE(s) INDEX"`
	UserID  int64 `xorm:"UNIQUE(s) INDEX"`
}

func (Member) TableName() string {
	return "user_group_member"
}

func init() {
	db.RegisterModel(new(Member))
}

// globalGroupMemberCount is a helper for a batch member-count query.
type globalGroupMemberCount struct {
	GroupID int64 `xorm:"group_id"`
	Count   int64
}

// GetUserGroupMemberCounts returns a map of group ID → member count for the given groups.
func GetUserGroupMemberCounts(ctx context.Context, groupIDs []int64) (map[int64]int64, error) {
	counts := make(map[int64]int64, len(groupIDs))
	if len(groupIDs) == 0 {
		return counts, nil
	}
	var rows []globalGroupMemberCount
	if err := db.GetEngine(ctx).Table("user_group_member").
		In("group_id", groupIDs).
		Select("group_id, COUNT(*) AS count").
		GroupBy("group_id").
		Find(&rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		counts[r.GroupID] = r.Count
	}
	return counts, nil
}

// ReplaceUserGroupMembers replaces members of a user group.
func ReplaceUserGroupMembers(ctx context.Context, groupID int64, userIDs []int64) error {
	uniqueIDs := container.Set[int64]{}
	uniqueIDs.AddMultiple(userIDs...)

	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Where("group_id=?", groupID).Delete(new(Member)); err != nil {
			return err
		}

		if len(uniqueIDs) == 0 {
			return nil
		}

		members := make([]Member, 0, len(uniqueIDs))
		for _, userID := range uniqueIDs.Values() {
			members = append(members, Member{
				GroupID: groupID,
				UserID:  userID,
			})
		}
		return db.Insert(ctx, &members)
	})
}

// AddUserToUserGroup adds a user to the user group.
func AddUserToUserGroup(ctx context.Context, groupID, userID int64) error {
	return db.Insert(ctx, &Member{GroupID: groupID, UserID: userID})
}

// RemoveUserFromUserGroup removes a user from the user group.
func RemoveUserFromUserGroup(ctx context.Context, groupID, userID int64) error {
	_, err := db.GetEngine(ctx).Delete(&Member{GroupID: groupID, UserID: userID})
	return err
}

// GetUserGroupMembers returns users directly in the user group.
func GetUserGroupMembers(ctx context.Context, groupID int64, listOpts db.ListOptions) ([]*user_model.User, error) {
	sess := db.GetEngine(ctx)
	if listOpts.PageSize > 0 && listOpts.Page > 0 {
		sess = sess.Limit(listOpts.PageSize, (listOpts.Page-1)*listOpts.PageSize)
	}

	members := make([]*user_model.User, 0, listOpts.PageSize)
	err := sess.In("id",
		builder.Select("user_id").
			From("user_group_member").
			Where(builder.Eq{"group_id": groupID}),
	).OrderBy(user_model.GetOrderByName()).Find(&members)
	return members, err
}

// IsUserInUserGroup returns true if user is directly in the user group.
func IsUserInUserGroup(ctx context.Context, groupID, userID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where("group_id=?", groupID).
		And("user_id=?", userID).
		Table("user_group_member").
		Exist()
}

// GetUserGroupIDsByUser returns group IDs the user is directly in.
func GetUserGroupIDsByUser(ctx context.Context, userID int64) ([]int64, error) {
	ids := make([]int64, 0, 10)
	if err := db.GetEngine(ctx).Table("user_group_member").
		Where("user_id=?", userID).
		Cols("group_id").Find(&ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// GetEffectiveUserGroupMemberIDs returns distinct user IDs for groups including their descendants.
func GetEffectiveUserGroupMemberIDs(ctx context.Context, groupIDs []int64) ([]int64, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}

	expandedIDs, err := ExpandUserGroupIDsToDescendants(ctx, groupIDs)
	if err != nil {
		return nil, err
	}

	userIDs := make([]int64, 0, 10)
	if err := db.GetEngine(ctx).Table("user_group_member").
		In("group_id", expandedIDs).
		Distinct("user_id").
		Find(&userIDs); err != nil {
		return nil, err
	}
	return userIDs, nil
}

// GetEffectiveUserGroupMembers returns distinct users for groups including their descendants.
func GetEffectiveUserGroupMembers(ctx context.Context, groupIDs []int64, listOpts db.ListOptions) ([]*user_model.User, error) {
	userIDs, err := GetEffectiveUserGroupMemberIDs(ctx, groupIDs)
	if err != nil || len(userIDs) == 0 {
		return nil, err
	}

	sess := db.GetEngine(ctx).In("id", userIDs).OrderBy(user_model.GetOrderByName())
	if listOpts.PageSize > 0 && listOpts.Page > 0 {
		sess = sess.Limit(listOpts.PageSize, (listOpts.Page-1)*listOpts.PageSize)
	}

	users := make([]*user_model.User, 0, listOpts.PageSize)
	if err := sess.Find(&users); err != nil {
		return nil, err
	}
	return users, nil
}
